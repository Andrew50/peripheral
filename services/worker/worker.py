#!/usr/bin/env python3
"""
Python Strategy Worker
Executes Python trading strategies in a sandboxed environment
"""

import asyncio
import json
import logging
import os
import sys
import time
import traceback
import uuid
from contextlib import contextmanager
from datetime import datetime
from io import StringIO
from typing import Any, Dict, Optional

import psutil
import redis

from src.data_provider import DataProvider
from src.execution_engine import PythonExecutionEngine
from src.security_validator import SecurityError, SecurityValidator

# Configure logging - console only
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    handlers=[
        logging.StreamHandler(sys.stdout),
        #logging.FileHandler("/app/logs/worker.log"),
    ],
)
logger = logging.getLogger(__name__)


class PythonWorker:
    """Python strategy execution worker"""

    def __init__(self):
        self.redis_client = self._init_redis()
        self.execution_engine = PythonExecutionEngine()
        self.security_validator = SecurityValidator()
        self.data_provider = DataProvider()
        self.worker_id = os.environ.get("HOSTNAME", str(uuid.uuid4()))

    def _init_redis(self) -> redis.Redis:
        """Initialize Redis connection"""
        redis_host = os.environ.get("REDIS_HOST", "localhost")
        redis_port = int(os.environ.get("REDIS_PORT", "6379"))
        redis_password = os.environ.get("REDIS_PASSWORD")

        return redis.Redis(
            host=redis_host,
            port=redis_port,
            password=redis_password,
            decode_responses=True,
            socket_connect_timeout=60,
            socket_timeout=60,
            socket_keepalive=True,
            socket_keepalive_options={},
            health_check_interval=30,
        )

    async def run(self):
        """Main worker loop"""
        logger.info(f"Python worker {self.worker_id} starting...")

        while True:
            try:
                # Listen for execution requests with longer timeout
                job_data = self.redis_client.blpop("python_execution_queue", timeout=60)

                if job_data:
                    _, job_json = job_data
                    job = json.loads(job_json)
                    await self.process_job(job)

            except KeyboardInterrupt:
                logger.info("Received shutdown signal")
                break
            except Exception as e:
                logger.error(f"Error in worker loop: {e}")
                # Add exponential backoff to prevent rapid retry loops
                await asyncio.sleep(5)

    async def process_job(self, job: Dict[str, Any]):
        """Process a single execution job"""
        execution_id = job.get("execution_id")
        logger.info(f"Processing job {execution_id}")

        try:
            # Update status to running
            await self._update_execution_status(
                execution_id,
                "running",
                {
                    "worker_node": self.worker_id,
                    "started_at": datetime.utcnow().isoformat(),
                },
            )

            # Extract job parameters
            python_code = job["python_code"]
            input_data = job.get("input_data", {})
            timeout_seconds = job.get("timeout_seconds", 300)
            memory_limit_mb = job.get("memory_limit_mb", 512)
            libraries = job.get("libraries", [])
            data_prep_sql = job.get("data_prep_sql")

            # Validate code security
            if not self.security_validator.validate_code(python_code):
                raise SecurityError("Code contains prohibited operations")

            # Prepare data if SQL provided
            prepared_data = None
            if data_prep_sql:
                prepared_data = await self.data_provider.execute_sql(data_prep_sql)

            # Execute the strategy
            start_time = time.time()
            result = await self._execute_with_limits(
                python_code=python_code,
                input_data=input_data,
                prepared_data=prepared_data,
                timeout_seconds=timeout_seconds,
                memory_limit_mb=memory_limit_mb,
                libraries=libraries,
            )
            execution_time_ms = int((time.time() - start_time) * 1000)

            # Update status to completed
            await self._update_execution_status(
                execution_id,
                "completed",
                {
                    "output_data": result,
                    "execution_time_ms": execution_time_ms,
                    "completed_at": datetime.utcnow().isoformat(),
                },
            )

            logger.info(
                f"Job {execution_id} completed successfully in {execution_time_ms}ms"
            )

        except SecurityError as e:
            logger.error(f"Security violation in job {execution_id}: {e}")
            await self._update_execution_status(
                execution_id,
                "failed",
                {
                    "error_message": f"Security violation: {str(e)}",
                    "completed_at": datetime.utcnow().isoformat(),
                },
            )

        except asyncio.TimeoutError:
            logger.error(f"Job {execution_id} timed out")
            await self._update_execution_status(
                execution_id,
                "timeout",
                {
                    "error_message": "Execution timed out",
                    "completed_at": datetime.utcnow().isoformat(),
                },
            )

        except MemoryError as e:
            logger.error(f"Job {execution_id} exceeded memory limit: {e}")
            await self._update_execution_status(
                execution_id,
                "failed",
                {
                    "error_message": f"Memory limit exceeded: {str(e)}",
                    "completed_at": datetime.utcnow().isoformat(),
                },
            )

        except Exception as e:
            logger.error(f"Job {execution_id} failed: {e}")
            await self._update_execution_status(
                execution_id,
                "failed",
                {
                    "error_message": str(e),
                    "error_traceback": traceback.format_exc(),
                    "completed_at": datetime.utcnow().isoformat(),
                },
            )

    async def _execute_with_limits(
        self,
        python_code: str,
        input_data: Dict[str, Any],
        prepared_data: Optional[Dict[str, Any]],
        timeout_seconds: int,
        memory_limit_mb: int,
        libraries: list,
    ) -> Dict[str, Any]:
        """Execute Python code with resource limits"""

        # Set up execution context
        execution_context = {
            "input_data": input_data,
            "prepared_data": prepared_data,
            "libraries": libraries,
        }

        # Monitor resource usage
        process = psutil.Process()
        initial_memory = process.memory_info().rss / 1024 / 1024  # MB

        try:
            # Execute with timeout
            result = await asyncio.wait_for(
                self.execution_engine.execute(python_code, execution_context),
                timeout=timeout_seconds,
            )

            # Check memory usage
            final_memory = process.memory_info().rss / 1024 / 1024  # MB
            memory_used = final_memory - initial_memory

            if memory_used > memory_limit_mb:
                raise MemoryError(
                    f"Memory limit exceeded: {memory_used:.1f}MB > {memory_limit_mb}MB"
                )

            # Add resource usage to result
            result["_execution_stats"] = {
                "memory_used_mb": memory_used,
                "cpu_percent": process.cpu_percent(),
            }

            return result

        except asyncio.TimeoutError:
            raise
        except Exception as e:
            logger.error(f"Execution error: {e}")
            raise

    async def _update_execution_status(
        self, execution_id: str, status: str, additional_data: Dict[str, Any] = None
    ):
        """Update execution status in database via Redis"""
        try:
            update_data = {
                "execution_id": execution_id,
                "status": status,
                "worker_id": self.worker_id,
                "timestamp": datetime.utcnow().isoformat(),
            }

            if additional_data:
                update_data.update(additional_data)

            # Publish update for backend to process
            self.redis_client.publish(
                "python_execution_updates", json.dumps(update_data)
            )

        except Exception as e:
            logger.error(f"Failed to update execution status: {e}")


class MemoryError(Exception):
    """Raised when memory limit is exceeded"""

    pass


async def main():
    """Main entry point"""
    worker = PythonWorker()
    await worker.run()


if __name__ == "__main__":
    asyncio.run(main())
