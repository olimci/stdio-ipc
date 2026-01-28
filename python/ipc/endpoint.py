from __future__ import annotations

import json
import queue
import threading
from itertools import count
from typing import Any, Callable, Dict, Optional


class Endpoint:
    def __init__(
        self,
        reader,
        writer,
        handler: Optional[Callable[[Any], Any]] = None,
    ) -> None:
        self._reader = reader
        self._writer = writer
        self._handler = handler

        self._write_lock = threading.Lock()
        self._pending_lock = threading.Lock()
        self._pending: Dict[int, queue.Queue] = {}
        self._next_id = count(1)

        self._closed = threading.Event()
        self._start_lock = threading.Lock()
        self._started = False

    def start(self) -> None:
        with self._start_lock:
            if self._started:
                return
            self._started = True
            thread = threading.Thread(target=self._read_loop, name="ipc-read", daemon=True)
            thread.start()

    def close(self) -> None:
        if self._closed.is_set():
            return
        self._closed.set()
        with self._pending_lock:
            pending = list(self._pending.items())
            self._pending.clear()
        for _, waiter in pending:
            waiter.put(RuntimeError("ipc: endpoint closed"))

    def wait_closed(self, timeout: Optional[float] = None) -> bool:
        return self._closed.wait(timeout=timeout)

    def call(self, payload: Any, timeout: Optional[float] = None) -> Any:
        msg_id = next(self._next_id)
        waiter: queue.Queue = queue.Queue(maxsize=1)

        with self._pending_lock:
            self._pending[msg_id] = waiter

        try:
            self._send({"type": "request", "id": msg_id, "payload": payload})
        except Exception:
            with self._pending_lock:
                self._pending.pop(msg_id, None)
            raise

        try:
            result = waiter.get(timeout=timeout)
        except queue.Empty as exc:
            with self._pending_lock:
                self._pending.pop(msg_id, None)
            raise TimeoutError("ipc: call timed out") from exc

        if isinstance(result, Exception):
            raise result
        return result

    def _read_loop(self) -> None:
        try:
            for line in self._iter_lines():
                try:
                    msg = json.loads(line)
                except json.JSONDecodeError:
                    continue

                msg_type = msg.get("type")
                if msg_type == "request":
                    threading.Thread(
                        target=self._handle_request,
                        args=(msg,),
                        name="ipc-handle",
                        daemon=True,
                    ).start()
                elif msg_type == "response":
                    self._handle_response(msg)
        finally:
            self.close()

    def _iter_lines(self):
        while not self._closed.is_set():
            line = self._reader.readline()
            if not line:
                return
            if isinstance(line, bytes):
                line = line.decode("utf-8", errors="replace")
            line = line.strip()
            if not line:
                continue
            yield line

    def _handle_request(self, msg: Dict[str, Any]) -> None:
        msg_id = msg.get("id")
        if msg_id is None:
            return

        if self._handler is None:
            self._send_error(msg_id, "ipc: no handler")
            return

        try:
            payload = msg.get("payload")
            result = self._handler(payload)
        except Exception as exc:
            self._send_error(msg_id, str(exc))
            return

        self._send({"type": "response", "id": msg_id, "payload": result})

    def _handle_response(self, msg: Dict[str, Any]) -> None:
        msg_id = msg.get("id")
        if msg_id is None:
            return

        with self._pending_lock:
            waiter = self._pending.pop(msg_id, None)

        if waiter is None:
            return

        error = msg.get("error")
        if error:
            message = error.get("message", "ipc: error") if isinstance(error, dict) else str(error)
            waiter.put(RuntimeError(message))
            return

        waiter.put(msg.get("payload"))

    def _send_error(self, msg_id: int, message: str) -> None:
        self._send({"type": "response", "id": msg_id, "error": {"message": message}})

    def _send(self, msg: Dict[str, Any]) -> None:
        line = json.dumps(msg, separators=(",", ":")) + "\n"
        with self._write_lock:
            try:
                self._writer.write(line)
            except TypeError:
                self._writer.write(line.encode("utf-8"))
            self._writer.flush()
