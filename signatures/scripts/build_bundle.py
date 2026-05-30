#!/usr/bin/env python3
"""
Build a signatures.json bundle from all YAML signature files.

Usage:
    python3 build_bundle.py <signatures_dir> <output_file>

The bundle format matches intelligence.SignatureStore:
{
  "version": 1,
  "updated_at": "<iso8601>",
  "signatures": [ { ...DetectionSignature... }, ... ]
}
"""

import sys
import os
import json
import re
import time
from datetime import datetime, timezone
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML not installed. Run: pip install pyyaml", file=sys.stderr)
    sys.exit(1)


VALID_TYPES = {
    "blocklisted_package",
    "typosquatting_target",
    "behavioral_rule",
    "malware_pattern",
    "mcp_injection_pattern",
    "pickle_rule",
}

VALID_ECOSYSTEMS = {"npm", "pypi", "go", "rubygems", "crates", "maven", "huggingface", "mcp", "*"}
VALID_SEVERITIES = {"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFORMATIONAL"}


def load_yaml_sig(path: Path) -> dict:
    with open(path) as f:
        data = yaml.safe_load(f)
    if not isinstance(data, dict):
        raise ValueError(f"YAML file does not contain a mapping: {path}")
    return data


def validate_sig(sig: dict, path: Path) -> list:
    errors = []
    for field in ("id", "name", "type", "ecosystem", "severity", "description"):
        if not sig.get(field):
            errors.append(f"missing required field: {field}")

    if sig.get("type") and sig["type"] not in VALID_TYPES:
        errors.append(f"invalid type: {sig['type']}")

    if sig.get("ecosystem") and sig["ecosystem"] not in VALID_ECOSYSTEMS:
        errors.append(f"invalid ecosystem: {sig['ecosystem']}")

    sev = str(sig.get("severity", "")).upper()
    if sev and sev not in VALID_SEVERITIES:
        errors.append(f"invalid severity: {sig['severity']}")

    t = sig.get("type", "")
    if t == "blocklisted_package" and not sig.get("package"):
        errors.append("blocklisted_package requires: package")
    if t == "typosquatting_target" and not sig.get("target"):
        errors.append("typosquatting_target requires: target")
    if t in ("malware_pattern", "mcp_injection_pattern") and not sig.get("pattern"):
        errors.append(f"{t} requires: pattern")
    if sig.get("pattern"):
        try:
            re.compile(sig["pattern"])
        except re.error as e:
            errors.append(f"invalid regex in pattern: {e}")
    if t in ("behavioral_rule", "pickle_rule") and not sig.get("rule"):
        errors.append(f"{t} requires: rule")

    return errors


def yaml_to_detection_sig(sig: dict) -> dict:
    """Convert YAML community format → DetectionSignature JSON format."""
    return {
        "id": sig.get("id", ""),
        "type": sig.get("type", ""),
        "ecosystem": sig.get("ecosystem", "*"),
        "target": sig.get("target", ""),
        "pattern": sig.get("pattern", ""),
        "package": sig.get("package", ""),
        "rule": sig.get("rule", ""),
        "severity": str(sig.get("severity", "HIGH")).upper(),
        "title": sig.get("name", ""),
        "description": sig.get("description", ""),
        "source": "community",
        "cve": sig.get("cve", ""),
        "created_at": datetime.now(timezone.utc).isoformat(),
    }


def build_bundle(sig_dir: str, out_file: str):
    sig_path = Path(sig_dir)
    yaml_files = list(sig_path.rglob("*.yaml")) + list(sig_path.rglob("*.yml"))
    yaml_files = [f for f in yaml_files if ".github" not in str(f)]

    if not yaml_files:
        print(f"No YAML files found in {sig_dir}", file=sys.stderr)
        sys.exit(1)

    signatures = []
    errors_found = []
    seen_ids = {}

    for path in sorted(yaml_files):
        try:
            sig = load_yaml_sig(path)
        except Exception as e:
            errors_found.append(f"{path}: {e}")
            continue

        errs = validate_sig(sig, path)
        if errs:
            for e in errs:
                errors_found.append(f"{path}: {e}")
            continue

        sig_id = sig.get("id", "")
        if sig_id in seen_ids:
            errors_found.append(f"{path}: duplicate ID '{sig_id}' (first seen in {seen_ids[sig_id]})")
            continue
        seen_ids[sig_id] = str(path)

        signatures.append(yaml_to_detection_sig(sig))

    if errors_found:
        print(f"\n{len(errors_found)} validation error(s):\n", file=sys.stderr)
        for e in errors_found:
            print(f"  ✗  {e}", file=sys.stderr)
        print(file=sys.stderr)
        sys.exit(1)

    bundle = {
        "version": 1,
        "updated_at": datetime.now(timezone.utc).isoformat(),
        "signatures": signatures,
    }

    out_path = Path(out_file)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with open(out_path, "w") as f:
        json.dump(bundle, f, indent=2)

    print(f"✓  {len(signatures)} signatures → {out_file}")

    # Print breakdown by type
    by_type = {}
    for s in signatures:
        t = s["type"]
        by_type[t] = by_type.get(t, 0) + 1
    for t, n in sorted(by_type.items()):
        print(f"   {n:4d}  {t}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <signatures_dir> <output_file>", file=sys.stderr)
        sys.exit(1)
    build_bundle(sys.argv[1], sys.argv[2])
