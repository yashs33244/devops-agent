"""Allow `python -m sre_guard.daemon` and `python -m sre_guard`."""

from sre_guard.daemon import run

if __name__ == "__main__":
    run()
