"""Executable policy fixture; the Markdown recovery workflow remains manual."""

import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest


def select_historical_landing(git, base, work, archive_path):
    work_tree = git("rev-parse", work + "^{tree}")
    work_archive = git("show", work + ":" + archive_path)
    if git("show", "main:" + archive_path) != work_archive:
        raise ValueError("archive bytes mismatch")
    candidates = []
    for line in git("rev-list", "--parents", "main").decode().splitlines():
        commit, *parents = line.split()
        if parents != [base] or git("rev-parse", commit + "^{tree}") != work_tree:
            continue
        archive = git("show", commit + ":" + archive_path)
        if archive != work_archive:
            raise ValueError("archive bytes mismatch")
        marker = b"\n\n<!-- blueprint:completion "
        if archive.count(marker) != 1:
            raise ValueError("malformed archive annotation")
        spec, annotation = archive.split(marker)
        if not annotation.endswith(b" -->\n"):
            raise ValueError("malformed archive annotation")
        record = json.loads(annotation[:-5])
        if (len(spec) != record["specBytes"]
                or hashlib.sha256(spec).hexdigest() != record["specSha256"]):
            raise ValueError("archive hash mismatch")
        candidates.append(commit)
    if len(candidates) != 1:
        raise ValueError("ambiguous landing" if candidates else "no matching landing")
    return candidates[0]


class HistoricalLandingWaiverTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="historical-landing-waiver-")
        self.addCleanup(temporary.cleanup)
        self.repo = Path(temporary.name)
        self.git("init", "--quiet", "--initial-branch=main")
        self.git("config", "user.name", "Waiver Fixture")
        self.git("config", "user.email", "fixture@example.invalid")
        self.git("-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null",
                 "commit", "--quiet", "--allow-empty", "-m", "base")
        self.base = self.git("rev-parse", "HEAD").decode().strip()
        self.archive_path = "archive.md"

    def git(self, *args):
        return subprocess.run(
            ["git", *args], cwd=self.repo, check=True,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        ).stdout

    def commit(self, tree, parents, message):
        args = ["-c", "commit.gpgsign=false", "commit-tree", tree]
        for parent in parents:
            args.extend(["-p", parent])
        return self.git(*args, "-m", message).decode().strip()

    def prepare(self, *, ambiguous=False, tampered=False):
        spec = b"# Verified fixture spec\n"
        annotation = {
            "specBytes": len(spec),
            "specSha256": hashlib.sha256(spec).hexdigest(),
        }
        stored_spec = spec.replace(b"Verified", b"Tampered") if tampered else spec
        archive = stored_spec + b"\n\n<!-- blueprint:completion "
        archive += json.dumps(annotation).encode() + b" -->\n"
        (self.repo / self.archive_path).write_bytes(archive)
        self.git("add", "--", self.archive_path)
        tree = self.git("write-tree").decode().strip()
        self.work = self.commit(tree, [self.base], "work")
        landing = self.commit(tree, [self.base], "landing one")
        parents = [landing]
        if ambiguous:
            other = self.commit(tree, [self.base], "landing two")
            self.assertNotEqual(landing, other)
            self.assertEqual(self.git("rev-parse", landing + "^{tree}"),
                             self.git("rev-parse", other + "^{tree}"))
            parents.append(other)
        tip = self.commit(tree, parents, "main advanced")
        self.git("update-ref", "refs/heads/main", tip)
        for parent in parents:
            self.git("merge-base", "--is-ancestor", parent, "main")
        return landing

    def test_accept(self):
        landing = self.prepare()
        self.assertEqual(
            select_historical_landing(self.git, self.base, self.work, self.archive_path),
            landing,
        )

    def test_stop_on_ambiguous(self):
        self.prepare(ambiguous=True)
        with self.assertRaisesRegex(ValueError, "ambiguous landing"):
            select_historical_landing(self.git, self.base, self.work, self.archive_path)

    def test_stop_on_tampered_archive_hash(self):
        self.prepare(tampered=True)
        with self.assertRaisesRegex(ValueError, "archive hash mismatch"):
            select_historical_landing(self.git, self.base, self.work, self.archive_path)


if __name__ == "__main__":
    unittest.main(verbosity=2)
