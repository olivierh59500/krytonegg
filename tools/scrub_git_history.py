#!/usr/bin/env python3
"""Remove local-only disk data from every Git revision without deleting local files.

Run --audit for a read-only check. --rewrite rewrites commit and tag objects,
expires their old reflogs and prunes unreachable objects. This is intended for
the explicitly requested cleanup of this standalone local repository.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile


def git(*arguments: str, data: bytes | None = None, env: dict | None = None,
        check: bool = True) -> bytes:
    result = subprocess.run(["git", *arguments], input=data, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, env=env)
    if check and result.returncode:
        raise RuntimeError(result.stderr.decode("utf-8", "replace"))
    return result.stdout


def forbidden(path: str) -> bool:
    parts = path.split("/")
    if ".DS_Store" in parts or parts[0] == "previous":
        return True
    if parts[0] == "assets":
        return not (len(parts) == 2 and
                    (parts[1].endswith(".go") or parts[1] == "README.md"))
    return path == "android/app/src/main/res/drawable-nodpi/ic_launcher.png"


def tree_entries(commit: str) -> list[tuple[bytes, str]]:
    entries = []
    for record in git("ls-tree", "-rz", "--full-tree", commit).split(b"\0"):
        if record:
            header, path = record.split(b"\t", 1)
            entries.append((header.split()[2], os.fsdecode(path)))
    return entries


def audit() -> dict:
    violations = []
    commits = git("rev-list", "--all").decode().splitlines()
    for commit in commits:
        for blob, path in tree_entries(commit):
            if forbidden(path):
                violations.append({"commit": commit, "path": path,
                                   "object": blob.decode()})
    return {"commits_checked": len(commits), "violations": violations}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--audit", action="store_true", help="only inspect reachable history")
    parser.add_argument("--rewrite", action="store_true", help="purge forbidden paths and objects")
    arguments = parser.parse_args()
    if arguments.audit == arguments.rewrite:
        parser.error("choose exactly one of --audit or --rewrite")
    root = Path(git("rev-parse", "--show-toplevel").decode().strip())
    os.chdir(root)
    before = audit()
    if arguments.audit:
        print(json.dumps(before, indent=2))
        raise SystemExit(bool(before["violations"]))
    if git("remote").strip():
        raise RuntimeError("This cleanup expects a standalone local repository without remotes")
    if Path(git("rev-parse", "--git-path", "objects/info/alternates").decode().strip()).exists():
        raise RuntimeError("Shared Git object stores need a separate cleanup strategy")
    forbidden_objects = {item["object"] for item in before["violations"]}
    old_head = git("rev-parse", "HEAD").decode().strip()
    refs = [line.split(" ", 1) for line in git(
        "for-each-ref", "--format=%(refname) %(objectname)").decode().splitlines()]
    mapping: dict[str, str] = {}
    with tempfile.TemporaryDirectory(prefix="krytonegg-git-index-") as temporary:
        env = dict(os.environ, GIT_INDEX_FILE=str(Path(temporary) / "index"))
        commits = git("rev-list", "--reverse", "--topo-order", "--all").decode().splitlines()
        for commit in commits:
            git("read-tree", commit, env=env)
            removed = [path for _, path in tree_entries(commit) if forbidden(path)]
            if removed:
                git("update-index", "--force-remove", "-z", "--stdin", env=env,
                    data=b"\0".join(os.fsencode(path) for path in removed) + b"\0")
            tree = git("write-tree", env=env).decode().strip()
            raw = git("cat-file", "commit", commit)
            header, message = raw.split(b"\n\n", 1)
            rewritten = []
            dropping_signature = False
            for line in header.splitlines():
                if line.startswith((b"gpgsig ", b"gpgsig-sha256 ")):
                    dropping_signature = True
                    continue
                if dropping_signature and line.startswith(b" "):
                    continue
                dropping_signature = False
                if line.startswith(b"tree "):
                    line = b"tree " + tree.encode()
                elif line.startswith(b"parent "):
                    parent = line[7:].decode()
                    line = b"parent " + mapping[parent].encode()
                rewritten.append(line)
            payload = b"\n".join(rewritten) + b"\n\n" + message
            mapping[commit] = git("hash-object", "-t", "commit", "-w", "--stdin",
                                  data=payload).decode().strip()

        def remap(object_id: str) -> str:
            if object_id in mapping:
                return mapping[object_id]
            if git("cat-file", "-t", object_id).strip() != b"tag":
                raise RuntimeError(f"Unsupported non-commit Git reference: {object_id}")
            raw = git("cat-file", "tag", object_id)
            first, remaining = raw.split(b"\n", 1)
            target = remap(first[7:].decode())
            # A rewritten signed tag cannot retain its old cryptographic signature.
            remaining = remaining.split(b"-----BEGIN PGP SIGNATURE-----", 1)[0]
            return git("hash-object", "-t", "tag", "-w", "--stdin",
                       data=b"object " + target.encode() + b"\n" + remaining).decode().strip()

        updates = [f"update {name} {remap(old)} {old}" for name, old in refs]
        git("update-ref", "--stdin", data=("\n".join(updates) + "\n").encode())
        # Reset only the index; the user's disk, extracted assets, and working
        # source modifications stay intact on the filesystem.
        git("reset", "--mixed", mapping[old_head])
    git("reflog", "expire", "--expire=now", "--expire-unreachable=now", "--all")
    git("gc", "--prune=now")
    remaining = audit()
    all_objects = set(git("cat-file", "--batch-all-objects", "--batch-check=%(objectname)")
                      .decode().splitlines())
    survivors = sorted(forbidden_objects & all_objects)
    if remaining["violations"] or survivors:
        raise RuntimeError(f"Cleanup verification failed: {remaining}, surviving objects={survivors}")
    print(json.dumps({"commits_rewritten": len(mapping),
                      "removed_history_entries": len(before["violations"]),
                      "purged_asset_objects": len(forbidden_objects),
                      "remaining_forbidden_paths": 0,
                      "remaining_forbidden_objects": 0,
                      "local_files_preserved": True}, indent=2))


if __name__ == "__main__":
    main()
