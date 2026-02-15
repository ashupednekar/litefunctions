import asyncio
import importlib
import inspect
import json
import logging
import os
import shutil
import sys
from types import ModuleType
from typing import Any, AsyncGenerator

import pygit2
import uvicorn
from fastapi import FastAPI, Request, Response
from nats.aio.msg import Msg

from pkg.conf import settings
from pkg.state import AppState

logger = logging.getLogger(__name__)
log_format = "%(asctime)s [%(levelname)s] - %(message)s"
date_format = "%Y-%m-%d %H:%M:%S"
logging.basicConfig(level=logging.INFO, format=log_format, datefmt=date_format)

MODULES: dict[str, ModuleType] = {}
MODULE_NAMES: set[str] = set()
CONSUMER_TASKS: dict[str, asyncio.Task] = {}
MODULE_LOCK = asyncio.Lock()

app = FastAPI()


def repo_path() -> str:
    return f"/tmp/{settings.project}"


def repo_functions_path() -> str:
    return os.path.join(repo_path(), "functions", "python")


def runtime_functions_path() -> str:
    return os.path.join(os.path.dirname(__file__), "pkg", "functions")


def repo_url() -> str:
    base = settings.vcs_base_url.rstrip("/")
    return f"{base}/{settings.git_user}/{settings.project}"


def git_callbacks() -> pygit2.RemoteCallbacks | None:
    if not settings.git_token:
        return None

    def _credentials(url: str, username_from_url: str | None, allowed_types: int):
        username = settings.git_user or username_from_url or "git"
        return pygit2.UserPass(username, settings.git_token)

    return pygit2.RemoteCallbacks(credentials=_credentials)


def sync_runtime_functions_dir(source_root: str) -> None:
    runtime_root = runtime_functions_path()
    os.makedirs(runtime_root, exist_ok=True)

    for entry in os.listdir(runtime_root):
        if not entry.endswith(".py") or entry == "__init__.py":
            continue
        try:
            os.remove(os.path.join(runtime_root, entry))
        except FileNotFoundError:
            pass

    if not os.path.isdir(source_root):
        return

    for entry in os.listdir(source_root):
        if not entry.endswith(".py"):
            continue
        if "__" in entry[:-3]:
            continue
        shutil.copy2(os.path.join(source_root, entry), os.path.join(runtime_root, entry))


def clear_module_cache() -> None:
    importlib.invalidate_caches()
    for name in list(MODULE_NAMES):
        sys.modules.pop(name, None)
    MODULE_NAMES.clear()
    MODULES.clear()


def _sync_repo() -> None:
    path = repo_path()
    callbacks = git_callbacks()
    if os.path.exists(path):
        repo = pygit2.Repository(path)
        remote = repo.remotes["origin"]
        if callbacks is not None:
            remote.fetch(callbacks=callbacks)
        else:
            remote.fetch()

        target_ref = None
        for ref_name in ("refs/remotes/origin/main", "refs/remotes/origin/master"):
            try:
                target_ref = repo.lookup_reference(ref_name)
                break
            except KeyError:
                continue

        if target_ref is None:
            raise RuntimeError("unable to find origin/main or origin/master after fetch")

        repo.set_head(target_ref.name)
        repo.checkout_head(strategy=pygit2.GIT_CHECKOUT_FORCE)
        return

    if callbacks is not None:
        pygit2.clone_repository(url=repo_url(), path=path, callbacks=callbacks)
    else:
        pygit2.clone_repository(url=repo_url(), path=path)


async def sync_repo_and_reload() -> None:
    logger.info("syncing repo project=%s repo_url=%s", settings.project, repo_url())
    await asyncio.to_thread(_sync_repo)
    source_root = repo_functions_path()
    await asyncio.to_thread(sync_runtime_functions_dir, source_root)
    clear_module_cache()
    logger.info(
        "repo sync complete project=%s repo_functions_path=%s runtime_functions_path=%s",
        settings.project,
        source_root,
        runtime_functions_path(),
    )


def list_functions() -> list[str]:
    root = runtime_functions_path()
    if not os.path.isdir(root):
        return []
    funcs: list[str] = []
    for entry in os.listdir(root):
        if not entry.endswith(".py"):
            continue
        name = entry[:-3]
        if "__" in name:
            continue
        funcs.append(name)
    return sorted(funcs)


async def load_module(name: str) -> ModuleType:
    async with MODULE_LOCK:
        cached = MODULES.get(name)
        if cached is not None:
            return cached

        file_path = os.path.join(runtime_functions_path(), f"{name}.py")
        if not os.path.exists(file_path):
            raise FileNotFoundError(file_path)

        spec = importlib.util.spec_from_file_location(name, file_path)
        if spec is None or spec.loader is None:
            raise RuntimeError(f"failed to load module spec for {name}")

        module = importlib.util.module_from_spec(spec)
        sys.modules[name] = module
        spec.loader.exec_module(module)
        MODULES[name] = module
        MODULE_NAMES.add(name)
        return module


class PubSubRequest:
    def __init__(self, body: bytes):
        self._body = body

    async def body(self) -> bytes:
        return self._body

    async def json(self) -> Any:
        if not self._body:
            return {}
        return json.loads(self._body.decode("utf-8"))


def encode_payload(value: Any) -> bytes:
    if value is None:
        return b""
    if isinstance(value, (bytes, bytearray)):
        return bytes(value)
    if hasattr(value, "model_dump"):
        return json.dumps(value.model_dump()).encode("utf-8")
    if isinstance(value, (dict, list, str, int, float, bool)):
        return json.dumps(value).encode("utf-8")
    if hasattr(value, "__dict__"):
        return json.dumps(value.__dict__).encode("utf-8")
    return str(value).encode("utf-8")


async def invoke_module(
    state: AppState, name: str, req_id: str, payload: bytes, request: Request | None = None
) -> AsyncGenerator[bytes, None]:
    module = await load_module(name)
    handler = getattr(module, "handle", None)
    if handler is None:
        raise RuntimeError(f"module '{name}' does not export handle")

    params = list(inspect.signature(handler).parameters.values())

    # Backward compatibility for old handlers: handle(state, req_id)
    if len(params) >= 2:
        result = handler(state, req_id)
    else:
        req_obj: Any = request if request is not None else PubSubRequest(payload)
        result = handler(req_obj)

    if inspect.isawaitable(result):
        result = await result

    if hasattr(result, "__aiter__"):
        async for item in result:
            yield encode_payload(item)
        return

    if inspect.isgenerator(result):
        for item in result:
            yield encode_payload(item)
        return

    yield encode_payload(result)


async def consume_function(state: AppState, name: str) -> None:
    subject = f"{settings.project}.{name}.exec.py.*"
    consumer = await state.js.pull_subscribe(subject, stream=settings.project)
    logger.info("python consumer started function=%s subject=%s", name, subject)

    while True:
        msgs = await consumer.fetch(timeout=None)
        msg: Msg = msgs[0]
        await msg.ack()
        req_id = msg.subject.split(".")[-1]
        try:
            async for out in invoke_module(state, name, req_id, msg.data):
                await state.nc.publish(f"{settings.project}.{name}.res.py.{req_id}", out)
        except Exception as exc:
            logger.exception("failed to handle async event for %s: %s", name, exc)


async def reconcile_consumers(state: AppState) -> None:
    desired = set(list_functions())

    for name in list(CONSUMER_TASKS.keys()):
        if name not in desired:
            task = CONSUMER_TASKS.pop(name)
            task.cancel()

    for name in desired:
        if name in CONSUMER_TASKS:
            continue
        CONSUMER_TASKS[name] = asyncio.create_task(consume_function(state, name))


async def watch_python_hook(state: AppState) -> None:
    subject = f"{settings.project}.hook.py"
    consumer = await state.js.pull_subscribe(subject, stream=settings.project)
    logger.info("python hook listener started project=%s subject=%s", settings.project, subject)
    while True:
        msgs = await consumer.fetch(timeout=None)
        msg: Msg = msgs[0]
        await msg.ack()
        try:
            logger.info(
                "python hook received project=%s subject=%s payload=%s",
                settings.project,
                msg.subject,
                msg.data.decode("utf-8", "ignore"),
            )
            await sync_repo_and_reload()
            await reconcile_consumers(state)
            logger.info("runtime code refreshed via hook project=%s", settings.project)
        except Exception as exc:
            logger.exception("failed to refresh runtime code: %s", exc)


@app.api_route("/{path:path}", methods=["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"])
async def handle_request(path: str, request: Request):
    state: AppState = app.state.state
    name = request.headers.get("X-Litefunction-Name") or getattr(settings, "name", "")
    if not name:
        return Response(content="function name not provided", status_code=400)

    body = await request.body()
    try:
        first = None
        async for out in invoke_module(state, name, "", body, request=request):
            first = out
            break
        return Response(content=first or b"", media_type="application/json", status_code=200)
    except FileNotFoundError:
        return Response(content=f"function '{name}' not found", status_code=404)
    except Exception as exc:
        logger.exception("sync invoke failed for %s: %s", name, exc)
        return Response(content=str(exc), status_code=500)


async def run() -> None:
    state = await AppState.new()
    app.state.state = state

    await sync_repo_and_reload()
    await reconcile_consumers(state)

    asyncio.create_task(watch_python_hook(state))

    config = uvicorn.Config(app, host="0.0.0.0", port=8080, log_level="info")
    server = uvicorn.Server(config)
    await server.serve()


if __name__ == "__main__":
    asyncio.run(run())
