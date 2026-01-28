from __future__ import annotations

from typing import Any, Callable, Dict, Optional


def new_request(kind: str, payload: Any) -> Dict[str, Any]:
    if not kind:
        raise ValueError("ipc: request type is empty")
    return {"type": kind, "payload": payload}


class Router:
    def __init__(self) -> None:
        self._routes: Dict[str, Callable[[Any], Any]] = {}
        self._fallback: Optional[Callable[[str, Any], Any]] = None

    def handle(self, kind: str, fn: Callable[[Any], Any]) -> None:
        if not kind:
            raise ValueError("ipc: route kind is empty")
        if fn is None:
            raise ValueError("ipc: nil handler")
        self._routes[kind] = fn

    def set_fallback(self, fn: Optional[Callable[[str, Any], Any]]) -> None:
        self._fallback = fn

    def handler(self) -> Callable[[Any], Any]:
        def _handler(payload: Any) -> Any:
            if not isinstance(payload, dict):
                raise ValueError("ipc: request payload must be an object")
            kind = payload.get("type")
            if not kind:
                raise ValueError("ipc: request type is empty")
            body = payload.get("payload")

            route = self._routes.get(kind)
            if route is not None:
                return route(body)
            if self._fallback is not None:
                return self._fallback(kind, body)
            raise ValueError(f"ipc: unknown route {kind!r}")

        return _handler


_ROUTES: Dict[str, Callable[[Any], Any]] = {}
_ON_START: list[Callable[[Any], Any]] = []


def Route(kind: str) -> Callable[[Callable[[Any], Any]], Callable[[Any], Any]]:
    if not kind:
        raise ValueError("ipc: route kind is empty")

    def decorator(fn: Callable[[Any], Any]) -> Callable[[Any], Any]:
        if fn is None:
            raise ValueError("ipc: nil handler")
        _ROUTES[kind] = fn
        return fn

    return decorator


def OnStart(fn: Callable[[Any], Any]) -> Callable[[Any], Any]:
    if fn is None:
        raise ValueError("ipc: nil handler")
    _ON_START.append(fn)
    return fn


def build_router() -> Router:
    router = Router()
    for kind, fn in _ROUTES.items():
        router.handle(kind, fn)
    return router


def serve_stdio() -> None:
    import sys
    import threading

    from .endpoint import Endpoint

    router = build_router()
    ep = Endpoint(sys.stdin, sys.stdout, router.handler())
    ep.start()

    for fn in list(_ON_START):
        threading.Thread(target=fn, args=(ep,), daemon=True).start()

    ep.wait_closed()


def clear_routes() -> None:
    _ROUTES.clear()
    _ON_START.clear()
