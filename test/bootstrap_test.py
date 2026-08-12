import os
import pathlib
import subprocess
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[1]


class BootstrapTest(unittest.TestCase):
    def test_rejects_old_go_before_npm(self):
        with tempfile.TemporaryDirectory() as directory:
            bindir = pathlib.Path(directory)
            go = bindir / "go"
            npm = bindir / "npm"
            go.write_text("#!/bin/sh\necho go1.21.13\n", encoding="utf-8")
            npm.write_text("#!/bin/sh\necho called > \"$MARKER\"\n", encoding="utf-8")
            go.chmod(0o755)
            npm.chmod(0o755)
            marker = bindir / "npm-called"
            env = os.environ | {
                "PATH": f"{bindir}:/usr/bin:/bin",
                "TASMANE_GO_BIN": str(go),
                "MARKER": str(marker),
            }
            result = subprocess.run(
                ["/bin/bash", "scripts/bootstrap.sh"], cwd=ROOT, env=env,
                capture_output=True, text=True, check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("Go 1.22 or newer", result.stderr)
            self.assertFalse(marker.exists())


if __name__ == "__main__":
    unittest.main()
