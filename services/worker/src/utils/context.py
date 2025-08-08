import time
import json
import threading
import logging
import traceback
from typing import Dict, Any, Optional
from datetime import datetime
from .conn import Conn

logger = logging.getLogger(__name__)

class NoSubscribersException(Exception):
    """Raised when a task has no subscribers."""
    pass


class Context():
    """
    Context is a class that provides a unified interface for all task execution contexts.
    It is used to publish progress updates, results, and heartbeats to the Redis pubsub system.
    """

    def __init__(self, conn: Conn, task_id: str, status_id: str, heartbeat_interval: int, queue_type: str, priority: str, worker_id: str, skip_heartbeat: bool = False):
        self.conn = conn
        self.start_time = time.time()
        self.task_id = task_id
        self.status_id = status_id
        self.heartbeat_interval = heartbeat_interval
        self.task_start_time = time.time()
        self.queue_type = queue_type
        self.priority = priority
        self.worker_id = worker_id
        self._cancellation_event = threading.Event()
        if not skip_heartbeat:
            self._start_heartbeat()
            try:
                self.publish_progress("running", {"worker_id": self.worker_id, "queue_type": queue_type, "priority": priority, "started_at": datetime.utcnow().isoformat()})
            except NoSubscribersException:
                logger.warning("Task %s has no subscribers on startup, signalling cancellation.", self.task_id)
                self._cancellation_event.set()

    def publish_progress(self, message: str, data: Optional[Dict[str, Any]] = None) -> None:
        """Publish progress updates for long-running tasks using unified messaging"""
        elapsed_time = time.time() - self.task_start_time

        update_data = data.copy() if data else {}
        update_data['elapsed_time'] = round(elapsed_time, 2)

        self._publish_update(
            message_type="progress",
            status=message,
            data=update_data
        )

    def _start_heartbeat(self) -> None:
        """Start the asynchronous heartbeat thread"""
        self._heartbeat_stop_event = threading.Event()
        self._heartbeat_thread = threading.Thread(target=self._heartbeat_loop, daemon=True)
        self._heartbeat_thread.start()


    # all status updates go through here
    def _publish_update(self, message_type: str, status: str, data: Dict[str, Any], error: Optional[Dict[str, str]] = None) -> None:
        """Publish status update"""
        elapsed_time = time.time() - self.task_start_time
        subscribers = self.conn.redis_client.publish(f"task_status:{self.status_id}", json.dumps({
            "task_id": self.task_id,
            "message_type": message_type,
            "status": status,
            "data": data,
            "elapsed_time": elapsed_time,
            "error": error
        }))
        if subscribers == 0:
            raise NoSubscribersException(f"No subscribers for task {self.task_id}")

    def _heartbeat_loop(self) -> None:
        """Asynchronous heartbeat loop - runs in separate thread"""

        while not self._heartbeat_stop_event.is_set():
            try:
                self._publish_update("heartbeat", "heartbeat", {})

            except NoSubscribersException:
                logger.warning("Task %s has no subscribers, signalling cancellation.", self.task_id)
                self._cancellation_event.set()
                break  # Stop heartbeat thread
            except Exception as e:
                logger.error("❌ Heartbeat thread error: %s", e)
            self._heartbeat_stop_event.wait(self.heartbeat_interval)

    def publish_result(self, results: Dict[str, Any], error: Optional[Dict[str, str]] = None, status: str = "completed") -> None:
        """Set task result in Redis and publish unified update"""
        # Calculate elapsed time since task initialization
        # Prepare data payload, include error if present
        payload = results.copy() if isinstance(results, dict) else {}

        try:
            self._publish_update(
                message_type="result",
                status=status,
                error=error,
                data=payload,
            )
        except NoSubscribersException:
            # It's ok if there are no subscribers when publishing final result.
            pass
        except Exception as e:
            logger.error("❌ Failed to publish result: %s", e)

    def destroy(self) -> None:
        """Destroy the execution context"""
        if hasattr(self, '_heartbeat_stop_event'):
            self._heartbeat_stop_event.set()
        if hasattr(self, '_heartbeat_thread'):
            self._heartbeat_thread.join(timeout=5)
        self.conn.redis_client.delete(f"task_status:{self.status_id}")
        self.conn.redis_client.delete(f"task_results:{self.task_id}")
        self.conn.redis_client.delete(f"task_heartbeats:{self.task_id}")
        self.conn.redis_client.delete(f"task_progress:{self.task_id}")

    def check_for_cancellation(self) -> None:
        """Check if task cancellation has been requested."""
        if self._cancellation_event.is_set():
            raise NoSubscribersException("Task cancelled due to no subscribers.")
