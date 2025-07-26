import time
import json
import threading
import logging
import traceback
from typing import Dict, Any
from datetime import datetime
from .conn import Conn

logger = logging.getLogger(__name__)

class NoSubscribersException(Exception):
    """Raised when a task has no subscribers."""
    pass


class Context():

    def __init__(self, conn: Conn, strategy_generator: StrategyGenerator, python_agent_generator: PythonAgentGenerator, task_id: str, status_id: str, heartbeat_interval: int, queue_type: str, priority: str, worker_id: str):
        self.conn = conn
        self.llm_client = co
        self.strategy_generator = strategy_generator
        self.python_agent_generator = python_agent_generator
        self.start_time = time.time()
        self.task_id = task_id
        self.status_id = status_id
        self.heartbeat_interval = heartbeat_interval
        self.task_start_time = time.time()
        self.queue_type = queue_type
        self.priority = priority
        self.worker_id = worker_id
        self._cancellation_event = threading.Event()
        self._start_heartbeat()
        try:
            self.publish_update("running", {"worker_id": self.worker_id, "queue_type": queue_type, "priority": priority, "started_at": datetime.utcnow().isoformat()})
        except NoSubscribersException:
            logger.warning(f"Task {self.task_id} has no subscribers on startup, signalling cancellation.")
            self._cancellation_event.set()

    def publish_update(self, message: str, data: Dict[str, Any] = None):
        """Publish progress updates for long-running tasks using unified messaging"""
        try:
            self._publish_status(
                message_type="update",
                status=message,
                data=data or {}
            )
        except Exception as e:
            logger.error(f"❌ Failed to publish progress for {self.task_id}: {e}")
            logger.error(f"📄 Full traceback: {traceback.format_exc()}")
            if isinstance(e, NoSubscribersException):
                raise

    def _start_heartbeat(self):
        """Start the asynchronous heartbeat thread"""
        self._heartbeat_stop_event = threading.Event()
        self._heartbeat_thread = threading.Thread(target=self._heartbeat_loop, daemon=True)
        self._heartbeat_thread.start()


    # all status updates go through here
    def _publish_status(self, message_type: str, status: str, data: Dict[str, Any]):
        """Publish status update"""
        subscribers = self.conn.redis_client.publish(f"task_status:{self.status_id}", json.dumps({
            "task_id": self.task_id,
            "message_type": message_type,
            "status": status,
            "data": data
        }))
        if subscribers == 0:
            raise NoSubscribersException(f"No subscribers for task {self.task_id}")

    def _heartbeat_loop(self):
        """Asynchronous heartbeat loop - runs in separate thread"""

        while not self._heartbeat_stop_event.is_set():
            try:
                self.publish_update("heartbeat", {})

            except NoSubscribersException:
                logger.warning(f"Task {self.task_id} has no subscribers, signalling cancellation.")
                self._cancellation_event.set()
                break  # Stop heartbeat thread
            except Exception as e:
                logger.error(f"❌ Heartbeat thread error: {e}")
            self._heartbeat_stop_event.wait(self.heartbeat_interval)

    def publish_result(self, results: Dict[str, Any], status: str, error: str = None):
        """Set task result in Redis and publish unified update"""
        # Prepare data payload, include error if present
        payload = results.copy() if isinstance(results, dict) else {}
        if error:
            payload['error'] = error
        try:
            self._publish_status(
                message_type="result",
                status=status,
                data=payload
            )
        except NoSubscribersException:
            # It's ok if there are no subscribers when publishing final result.
            pass
        except Exception as e:
            logger.error(f"❌ Failed to publish result: {e}")

    def destroy(self):
        """Destroy the execution context"""
        if hasattr(self, '_heartbeat_stop_event'):
            self._heartbeat_stop_event.set()
        if hasattr(self, '_heartbeat_thread'):
            self._heartbeat_thread.join(timeout=5)
            #logger.info("💓 Stopped heartbeat thread")
        self.conn.redis_client.delete(f"task_status:{self.status_id}")
        self.conn.redis_client.delete(f"task_results:{self.task_id}")
        self.conn.redis_client.delete(f"task_heartbeats:{self.task_id}")
        self.conn.redis_client.delete(f"task_progress:{self.task_id}")

    def check_for_cancellation(self):
        """Check if task cancellation has been requested."""
        if self._cancellation_event.is_set():
            raise NoSubscribersException("Task cancelled due to no subscribers.")
