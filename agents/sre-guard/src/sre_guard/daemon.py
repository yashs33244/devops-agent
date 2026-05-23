"""Daemon — asyncio entrypoint that starts MonitorLoop + FastAPI CommandServer."""

from __future__ import annotations

import asyncio
import logging
import os
import signal
import sys
from pathlib import Path
from typing import Optional

import uvicorn

from .commander import app as fastapi_app
from .commander import bind as bind_commander
from .config import SREGuardConfig, load_config
from .monitor import MonitorLoop

logger = logging.getLogger(__name__)

_PID_FILE = Path("/tmp/sre-guard.pid")


def _configure_logging(level: str) -> None:
    numeric = getattr(logging, level.upper(), logging.INFO)
    logging.basicConfig(
        level=numeric,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
        datefmt="%Y-%m-%dT%H:%M:%SZ",
        stream=sys.stdout,
    )


def _write_pid() -> None:
    try:
        _PID_FILE.write_text(str(os.getpid()))
    except OSError as exc:
        logger.warning("Could not write PID file %s: %s", _PID_FILE, exc)


def _remove_pid() -> None:
    try:
        _PID_FILE.unlink(missing_ok=True)
    except OSError:
        pass


async def main(config_path: Optional[str] = None) -> None:
    """
    Main coroutine — loads config, starts MonitorLoop and FastAPI, handles shutdown.
    """
    config = load_config(config_path)
    _configure_logging(config.log_level)
    _write_pid()

    logger.info(
        "SRE Guard starting — PID %d | port %d | poll %ds | services: %d",
        os.getpid(),
        config.api_port,
        config.poll_interval_seconds,
        len(config.services),
    )

    monitor = MonitorLoop(config)
    bind_commander(monitor, config)

    loop = asyncio.get_running_loop()
    shutdown_event = asyncio.Event()

    def _handle_signal(sig: signal.Signals) -> None:
        logger.info("Received %s — initiating graceful shutdown.", sig.name)
        shutdown_event.set()

    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, _handle_signal, sig)

    # Build uvicorn server (does NOT call run() — we drive it manually)
    uv_config = uvicorn.Config(
        app=fastapi_app,
        host="0.0.0.0",
        port=config.api_port,
        log_level=config.log_level.lower(),
        access_log=False,
    )
    server = uvicorn.Server(uv_config)

    monitor_task = asyncio.create_task(monitor.run(), name="monitor-loop")
    api_task = asyncio.create_task(server.serve(), name="api-server")
    shutdown_task = asyncio.create_task(shutdown_event.wait(), name="shutdown-sentinel")

    try:
        done, pending = await asyncio.wait(
            {monitor_task, api_task, shutdown_task},
            return_when=asyncio.FIRST_COMPLETED,
        )

        # If one of the real tasks finished unexpectedly (not the shutdown sentinel),
        # log and propagate any exception.
        for task in done:
            if task is not shutdown_task:
                exc = task.exception()
                if exc:
                    logger.error("Task %s raised: %s", task.get_name(), exc)
    finally:
        logger.info("Shutting down tasks…")
        server.should_exit = True

        for task in (monitor_task, api_task):
            if not task.done():
                task.cancel()
                try:
                    await task
                except (asyncio.CancelledError, Exception):
                    pass

        shutdown_task.cancel()
        _remove_pid()
        logger.info("SRE Guard stopped.")


def run(config_path: Optional[str] = None) -> None:
    """Synchronous entrypoint — called by CLI `sreguard daemon start`."""
    try:
        asyncio.run(main(config_path))
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    run()
