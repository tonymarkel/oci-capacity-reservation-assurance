#!/usr/bin/env python3
"""Poll an OCI capacity reservation until reserved-count matches the request."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import time
from decimal import Decimal, InvalidOperation
from typing import Any


TERMINAL_BAD_STATES = {"DELETED", "DELETING"}
IN_PROGRESS_STATES = {"CREATING", "UPDATING", "MOVING"}


def env(name: str, default: str | None = None) -> str:
    value = os.environ.get(name, default)
    if value is None or value == "":
        raise SystemExit(f"missing required environment variable: {name}")
    return value


def truthy(value: str) -> bool:
    return value.strip().lower() in {"1", "true", "yes", "y"}


def pick(mapping: dict[str, Any], *names: str, default: Any = None) -> Any:
    for name in names:
        if name in mapping:
            return mapping[name]
    return default


def number_key(value: Any) -> str:
    if value is None:
        return ""
    try:
        decimal = Decimal(str(value)).normalize()
    except (InvalidOperation, ValueError):
        return str(value)
    text = format(decimal, "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return text


def int_value(value: Any) -> int:
    if value is None:
        return 0
    return int(Decimal(str(value)))


def shape_config(config: dict[str, Any]) -> dict[str, Any] | None:
    return pick(config, "instanceShapeConfig", "instance_shape_config", "instance-shape-config")


def cluster_config(config: dict[str, Any]) -> dict[str, Any] | None:
    raw = pick(config, "clusterConfig", "cluster_config", "cluster-config")
    if raw is None:
        return None
    return {
        "hpcIslandId": pick(raw, "hpcIslandId", "hpc_island_id", "hpc-island-id"),
        "networkBlockIds": pick(raw, "networkBlockIds", "network_block_ids", "network-block-ids", default=[]),
    }


def config_key(config: dict[str, Any]) -> tuple[str, str, str, str, str, str]:
    sc = shape_config(config) or {}
    cc = cluster_config(config)
    return (
        str(pick(config, "instanceShape", "instance_shape", "instance-shape", default="")),
        number_key(pick(sc, "ocpus")),
        number_key(pick(sc, "memoryInGBs", "memory_in_gbs", "memory-in-gbs")),
        str(pick(config, "faultDomain", "fault_domain", "fault-domain", default="") or ""),
        str(pick(config, "clusterPlacementGroupId", "cluster_placement_group_id", "cluster-placement-group-id", default="") or ""),
        "" if cc is None else json.dumps(cc, sort_keys=True, separators=(",", ":")),
    )


def describe_config(config: dict[str, Any]) -> str:
    key = config_key(config)
    parts = [f"shape={key[0]}"]
    if key[1]:
        parts.append(f"ocpus={key[1]}")
    if key[2]:
        parts.append(f"memoryInGBs={key[2]}")
    if key[3]:
        parts.append(f"faultDomain={key[3]}")
    if key[4]:
        parts.append(f"clusterPlacementGroupId={key[4]}")
    return " ".join(parts)


def counts_by_config(configs: list[dict[str, Any]]) -> tuple[dict[tuple[str, str, str, str, str, str], int], dict[tuple[str, str, str, str, str, str], str]]:
    counts: dict[tuple[str, str, str, str, str, str], int] = {}
    labels: dict[tuple[str, str, str, str, str, str], str] = {}
    for config in configs:
        key = config_key(config)
        count = int_value(pick(config, "reservedCount", "reserved_count", "reserved-count"))
        counts[key] = counts.get(key, 0) + count
        labels[key] = describe_config(config)
    return counts, labels


def compare(
    requested: list[dict[str, Any]], actual: list[dict[str, Any]]
) -> tuple[bool, int, int, list[str]]:
    requested_counts, requested_labels = counts_by_config(requested)
    actual_counts, actual_labels = counts_by_config(actual)
    requested_total = sum(requested_counts.values())
    actual_total = sum(actual_counts.values())
    mismatches: list[str] = []

    for key, requested_count in requested_counts.items():
        actual_count = actual_counts.get(key, 0)
        if actual_count != requested_count:
            mismatches.append(
                f"{requested_labels[key]} requested reserved-count={requested_count} actual reserved-count={actual_count}"
            )

    for key, actual_count in actual_counts.items():
        if key not in requested_counts and actual_count:
            mismatches.append(f"unexpected {actual_labels[key]} actual reserved-count={actual_count}")

    return not mismatches, requested_total, actual_total, mismatches


def oci_base_args() -> list[str]:
    return [
        env("OCI_CLI_PATH", "oci"),
        "--config-file",
        os.path.expanduser(env("OCI_CONFIG_FILE", "~/.oci/config")),
        "--profile",
        env("OCI_PROFILE", "DEFAULT"),
    ]


def run_oci(args: list[str], check: bool = True) -> subprocess.CompletedProcess[str]:
    completed = subprocess.run(
        oci_base_args() + args,
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if check and completed.returncode != 0:
        sys.stderr.write(completed.stderr)
        raise SystemExit(completed.returncode)
    return completed


def get_reservation(reservation_id: str) -> dict[str, Any]:
    completed = run_oci(
        [
            "compute",
            "capacity-reservation",
            "get",
            "--capacity-reservation-id",
            reservation_id,
            "--output",
            "json",
        ]
    )
    return json.loads(completed.stdout)["data"]


def update_reservation(
    reservation_id: str,
    requested_configs: list[dict[str, Any]],
    poll_interval: int,
    max_wait_seconds: int,
) -> bool:
    with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as handle:
        json.dump(requested_configs, handle)
        handle.flush()
        payload_path = handle.name

    try:
        completed = run_oci(
            [
                "compute",
                "capacity-reservation",
                "update",
                "--capacity-reservation-id",
                reservation_id,
                "--instance-reservation-configs",
                f"file://{payload_path}",
                "--force",
                "--wait-for-state",
                "SUCCEEDED",
                "--wait-interval-seconds",
                str(poll_interval),
                "--max-wait-seconds",
                str(max_wait_seconds),
                "--output",
                "json",
            ],
            check=False,
        )
        if completed.returncode != 0:
            sys.stderr.write(completed.stderr)
            return False
        return True
    finally:
        try:
            os.unlink(payload_path)
        except OSError:
            pass


def sleep_or_timeout(deadline: float, poll_interval: int) -> None:
    remaining = deadline - time.monotonic()
    if remaining <= 0:
        raise TimeoutError
    time.sleep(min(poll_interval, remaining))


def main() -> int:
    reservation_id = env("RESERVATION_ID")
    requested_configs = json.loads(env("REQUESTED_CONFIGS_JSON"))
    poll_interval = int(env("POLL_INTERVAL_SECONDS", "30"))
    timeout_seconds = int(env("TIMEOUT_SECONDS", "1800"))
    update_until_match = truthy(env("UPDATE_UNTIL_MATCH", "true"))
    deadline = time.monotonic() + timeout_seconds

    print(f"Checking reserved-count values for capacity reservation {reservation_id}")
    while True:
        reservation = get_reservation(reservation_id)
        lifecycle_state = pick(reservation, "lifecycle-state", "lifecycleState", default="")
        actual_configs = pick(reservation, "instance-reservation-configs", "instanceReservationConfigs", default=[])
        matches, requested_total, actual_total, mismatches = compare(requested_configs, actual_configs)

        print(
            f"Reservation check: lifecycle={lifecycle_state} requested={requested_total} reserved-count={actual_total}",
            flush=True,
        )
        if matches:
            print("Validation passed: reserved-count values match requested configs")
            return 0

        for mismatch in mismatches:
            print(f"Config mismatch: {mismatch}", flush=True)

        if lifecycle_state in TERMINAL_BAD_STATES:
            raise SystemExit(f"reservation entered terminal state {lifecycle_state}")

        if lifecycle_state in IN_PROGRESS_STATES:
            sleep_or_timeout(deadline, poll_interval)
            continue

        if update_until_match:
            remaining = max(1, int(deadline - time.monotonic()))
            print("Updating reservation back to requested configs", flush=True)
            if not update_reservation(reservation_id, requested_configs, poll_interval, remaining):
                sleep_or_timeout(deadline, poll_interval)
            continue

        sleep_or_timeout(deadline, poll_interval)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except TimeoutError:
        raise SystemExit("timed out waiting for reserved-count values to match requested configs")
