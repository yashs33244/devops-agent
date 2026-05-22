#!/usr/bin/env python3
"""Detect app language and generate/validate a production Dockerfile."""

import argparse
import json
import subprocess
import sys
from pathlib import Path

TEMPLATES_DIR = Path(__file__).parent.parent / "templates" / "dockerfiles"

LANG_SIGNALS = {
    "node": ["package.json"],
    "python": ["requirements.txt", "pyproject.toml", "setup.py", "setup.cfg"],
    "go": ["go.mod"],
    "java": ["pom.xml", "build.gradle", "build.gradle.kts"],
    "rust": ["Cargo.toml"],
}


def detect_language(repo_path: Path) -> str | None:
    for lang, signals in LANG_SIGNALS.items():
        if any((repo_path / s).exists() for s in signals):
            return lang
    return None


def get_entrypoint(repo_path: Path, lang: str) -> dict:
    """Try to infer the app entrypoint from common config files."""
    info = {}
    if lang == "node":
        pkg = repo_path / "package.json"
        if pkg.exists():
            import json as _json
            data = _json.loads(pkg.read_text())
            info["name"] = data.get("name", "app")
            info["start"] = data.get("scripts", {}).get("start", "node index.js")
            info["port"] = 3000
    elif lang == "python":
        for candidate in ["main.py", "app.py", "server.py", "wsgi.py", "asgi.py"]:
            if (repo_path / candidate).exists():
                info["main"] = candidate
                break
        info["port"] = 8000
    elif lang == "go":
        info["port"] = 8080
    elif lang == "java":
        info["port"] = 8080
    return info


def render_template(template_path: Path, substitutions: dict) -> str:
    content = template_path.read_text()
    for key, value in substitutions.items():
        content = content.replace(f"{{{{{key}}}}}", str(value))
    return content


def build_image(repo_path: Path, service_name: str) -> tuple[bool, str]:
    result = subprocess.run(
        ["docker", "build", "-t", f"{service_name}:devops-agent-test", "."],
        cwd=repo_path,
        capture_output=True,
        text=True,
        timeout=300,
    )
    return result.returncode == 0, result.stdout + result.stderr


def main():
    parser = argparse.ArgumentParser(description="Dockerize a repository")
    parser.add_argument("--path", required=True, help="Path to the repository")
    parser.add_argument("--service", required=True, help="Service name (slug)")
    parser.add_argument("--force", action="store_true", help="Overwrite existing Dockerfile")
    parser.add_argument("--no-build", action="store_true", help="Skip docker build step")
    args = parser.parse_args()

    repo_path = Path(args.path).resolve()
    if not repo_path.exists():
        print(json.dumps({"success": False, "error": f"Path not found: {repo_path}"}))
        sys.exit(1)

    dockerfile_path = repo_path / "Dockerfile"

    # Check if Dockerfile already exists
    if dockerfile_path.exists() and not args.force:
        print(f"[dockerize] Existing Dockerfile found at {dockerfile_path}")
        if not args.no_build:
            ok, output = build_image(repo_path, args.service)
            print(json.dumps({
                "success": ok,
                "dockerfile_path": str(dockerfile_path),
                "image_name": f"{args.service}:devops-agent-test",
                "language": "existing",
                "build_output": output[-2000:] if not ok else "Build succeeded",
                "action": "validated_existing",
            }))
        else:
            print(json.dumps({
                "success": True,
                "dockerfile_path": str(dockerfile_path),
                "language": "existing",
                "action": "skipped_build",
            }))
        return

    # Detect language
    lang = detect_language(repo_path)
    if not lang:
        print(json.dumps({
            "success": False,
            "error": "Could not detect language. No package.json, requirements.txt, go.mod, pom.xml, or Cargo.toml found.",
            "hint": "Create a Dockerfile manually or add a language manifest file.",
        }))
        sys.exit(1)

    print(f"[dockerize] Detected language: {lang}")

    # Copy template
    template_dir = TEMPLATES_DIR / lang
    template_file = template_dir / "Dockerfile"
    if not template_file.exists():
        print(json.dumps({"success": False, "error": f"No template for language: {lang}"}))
        sys.exit(1)

    entrypoint = get_entrypoint(repo_path, lang)
    # Derive binary name from service slug (e.g. "go-api" → "go-api")
    binary_name = args.service if args.service else "server"
    dockerfile_content = render_template(template_file, {
        "SERVICE_NAME": args.service,
        "PORT": entrypoint.get("port", 8080),
        "MAIN_FILE": entrypoint.get("main", "main.py"),
        "START_CMD": entrypoint.get("start", "node index.js"),
        "BINARY_NAME": binary_name,
    })

    dockerfile_path.write_text(dockerfile_content)
    print(f"[dockerize] Wrote Dockerfile to {dockerfile_path}")

    if args.no_build:
        print(json.dumps({
            "success": True,
            "dockerfile_path": str(dockerfile_path),
            "image_name": f"{args.service}:devops-agent-test",
            "language": lang,
            "action": "generated",
        }))
        return

    # Build and test
    print(f"[dockerize] Building image {args.service}:devops-agent-test ...")
    ok, output = build_image(repo_path, args.service)
    if not ok:
        print(f"[dockerize] Build FAILED. Output:\n{output[-3000:]}")
    else:
        print("[dockerize] Build succeeded.")

    print(json.dumps({
        "success": ok,
        "dockerfile_path": str(dockerfile_path),
        "image_name": f"{args.service}:devops-agent-test",
        "language": lang,
        "action": "generated_and_built",
        "build_output": output[-2000:] if not ok else "Build succeeded",
    }))


if __name__ == "__main__":
    main()
