import time, psycopg2, redis, os, sys, socket, json
from datetime import datetime, timedelta
import random


# Global task context for logging
CURRENT_TASK_DATA = None
CURRENT_TASK_ID = None


def set_task_context(task_data, task_id):
    """Set the current task context for logging"""
    global CURRENT_TASK_DATA, CURRENT_TASK_ID
    CURRENT_TASK_DATA = task_data
    CURRENT_TASK_ID = task_id


def get_timestamp():
    """Get current timestamp in a standard format"""
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def log_message(message, level="info"):
    """Log a message to both stdout and task logs if task context is available"""
    # Always print to stdout for direct console viewing
    print(message, flush=True)
    
    # If we have a task context, add to task logs
    if CURRENT_TASK_DATA is not None and CURRENT_TASK_ID is not None:
        add_task_log(CURRENT_TASK_DATA, CURRENT_TASK_ID, message, level)


def add_task_log(data, task_id, message, level="info"):
    """Add a log entry to a task in Redis"""
    # Use a function attribute to track missing tasks we've already warned about
    if not hasattr(add_task_log, 'warned_missing_tasks'):
        add_task_log.warned_missing_tasks = set()
        
    try:
        # Get the current task
        task_json = safe_redis_operation(data.cache.get, task_id)
        if not task_json:
            # Only print warning if we haven't warned about this task before
            if task_id not in add_task_log.warned_missing_tasks:
                sys.__stdout__.write(f"Warning: Could not find task {task_id} to add log\n")
                sys.__stdout__.flush()
                add_task_log.warned_missing_tasks.add(task_id)
            return
        # If we successfully get the task, remove it from the warned set if it was there
        if task_id in add_task_log.warned_missing_tasks:
            add_task_log.warned_missing_tasks.remove(task_id)
        
        task = json.loads(task_json)
        
        # DEBUG: Print task structure before adding log - using sys.__stdout__ directly to avoid recursion
        sys.__stdout__.write(f"DEBUG: Task structure before log: {list(task.keys())}\n")
        sys.__stdout__.flush()
        
        # Create a log entry
        log_entry = {
            "timestamp": datetime.now().isoformat(),
            "message": message,
            "level": level
        }
        
        # Add the log entry to the task
        if "logs" not in task:
            task["logs"] = []
            sys.__stdout__.write(f"DEBUG: Created new logs array for task {task_id}\n")
            sys.__stdout__.flush()
        
        task["logs"].append(log_entry)
        task["updatedAt"] = datetime.now().isoformat()
        
        # DEBUG: Print log entry being added - using sys.__stdout__ directly to avoid recursion
        sys.__stdout__.write(f"DEBUG: Adding log to task {task_id}: {log_entry}\n")
        sys.__stdout__.write(f"DEBUG: Task now has {len(task['logs'])} logs\n")
        sys.__stdout__.flush()
        
        # Save the updated task
        safe_redis_operation(data.cache.set, task_id, json.dumps(task))
    except Exception as e:
        sys.__stdout__.write(f"Error adding log to task {task_id}: {e}\n")
        sys.__stdout__.flush()


def safe_redis_operation(func, *args, **kwargs):
    """Execute a Redis operation with retry logic for various failures"""
    max_retries = 5
    base_retry_delay = 1  # Base delay in seconds
    backoff_factor = 2    # Exponential backoff factor
    jitter = 0.2          # Random jitter to avoid thundering herd
    
    for attempt in range(max_retries):
        try:
            return func(*args, **kwargs)
        except (redis.TimeoutError, socket.timeout) as e:
            # Handle timeout errors with exponential backoff
            retry_delay = base_retry_delay * (backoff_factor ** attempt)
            jitter_amount = retry_delay * random.uniform(-jitter, jitter)
            total_delay = retry_delay + jitter_amount
            
            if attempt < max_retries - 1:
                sys.__stdout__.write(f"Redis operation timed out (attempt {attempt+1}/{max_retries}): {e}. Retrying in {total_delay:.2f}s\n")
                sys.__stdout__.flush()
                
                # Try to reset the connection pool if possible
                try:
                    if hasattr(func, '__self__') and hasattr(func.__self__, 'connection_pool'):
                        sys.__stdout__.write("Attempting to reset Redis connection pool...\n")
                        sys.__stdout__.flush()
                        func.__self__.connection_pool.reset()
                except Exception as reset_error:
                    sys.__stdout__.write(f"Failed to reset connection pool: {reset_error}\n")
                    sys.__stdout__.flush()
                
                time.sleep(total_delay)
            else:
                sys.__stdout__.write(f"Redis operation timed out after {max_retries} attempts: {e}\n")
                sys.__stdout__.flush()
                raise
        except (redis.ConnectionError, redis.exceptions.ConnectionError) as e:
            # Handle connection errors with exponential backoff
            retry_delay = base_retry_delay * (backoff_factor ** attempt)
            jitter_amount = retry_delay * random.uniform(-jitter, jitter)
            total_delay = retry_delay + jitter_amount
            
            if attempt < max_retries - 1:
                sys.__stdout__.write(f"Redis connection error (attempt {attempt+1}/{max_retries}): {e}. Retrying in {total_delay:.2f}s\n")
                sys.__stdout__.flush()
                time.sleep(total_delay)
            else:
                sys.__stdout__.write(f"Redis connection error after {max_retries} attempts: {e}\n")
                sys.__stdout__.flush()
                raise
        except Exception as e:
            # For other errors, just propagate them up
            sys.__stdout__.write(f"Unexpected Redis error: {e}\n")
            sys.__stdout__.flush()
            if attempt < max_retries - 1:
                sys.__stdout__.write(f"Retrying in {base_retry_delay:.2f}s\n")
                sys.__stdout__.flush()
                time.sleep(base_retry_delay)
            else:
                raise
from gemini import GeminiKeyPool



class Conn:
    def __init__(self, inside_container=True):
        if inside_container:
            cache_host = os.environ.get("REDIS_HOST", "cache")
            db_host = os.environ.get("DB_HOST", "db")
            tf_host = "http://tf:8501/"
        else:
            cache_host = "localhost"
            db_host = "localhost"
            tf_host = "http://localhost:8501/"
        
        # Get retry configuration from environment variables or use defaults
        self.redis_max_retries = int(os.environ.get("REDIS_RETRY_ATTEMPTS", "5"))
        self.redis_retry_delay = int(os.environ.get("REDIS_RETRY_DELAY", "1"))
        
        # Initial connection with 1-minute timeout
        start_time = datetime.now()
        timeout = timedelta(minutes=1)
        last_db_error = None
        last_redis_error = None
        
        while datetime.now() - start_time < timeout:
            try:
                # Try to establish both connections
                self._connect_to_db(db_host)
                self._connect_to_redis(cache_host)
                
                # If we get here, both connections were successful
                print("Successfully connected to both database and Redis", flush=True)
                self.tf = tf_host
                self.polygon = os.environ.get("POLYGON_API_KEY", "")
                
                # Initialize Gemini API key pool
                self.gemini_pool = GeminiKeyPool()
                
                return
                
            except psycopg2.OperationalError as e:
                last_db_error = str(e)
                print(f"Database connection failed: {e}", flush=True)
                time.sleep(2)  # Short sleep before retry
                
            except redis.ConnectionError as e:
                last_redis_error = str(e)
                print(f"Redis connection failed: {e}", flush=True)
                time.sleep(2)  # Short sleep before retry
                
            except Exception as e:
                print(f"Unexpected error during connection: {e}", flush=True)
                time.sleep(2)  # Short sleep before retry
        
        # If we get here, we've timed out
        error_msg = "Failed to establish connections within 1 minute:\n"
        if last_db_error:
            error_msg += f"Database error: {last_db_error}\n"
        if last_redis_error:
            error_msg += f"Redis error: {last_redis_error}"
        print(error_msg, flush=True)
        sys.exit(1)

    def _connect_to_db(self, db_host):
        # Get database credentials from environment variables
        db_port = os.environ.get("DB_PORT", "5432")
        db_user = os.environ.get("DB_USER", "postgres")
        db_password = os.environ.get("DB_PASSWORD", "")
        
        self.db = psycopg2.connect(
            host=db_host, 
            port=db_port, 
            user=db_user, 
            password=db_password,
            connect_timeout=5  # Short timeout for individual connection attempts
        )
        # Test the connection
        with self.db.cursor() as cursor:
            cursor.execute("SELECT 1")
    
    def _connect_to_redis(self, cache_host):
        # Get Redis configuration from environment variables
        redis_port = int(os.environ.get("REDIS_PORT", "6379"))
        redis_password = os.environ.get("REDIS_PASSWORD", "")
        redis_db = int(os.environ.get("REDIS_DB", "0"))
        
        # Get socket timeout values from environment variables or use defaults
        socket_timeout = float(os.environ.get("REDIS_SOCKET_TIMEOUT", "10.0"))
        socket_connect_timeout = float(os.environ.get("REDIS_SOCKET_CONNECT_TIMEOUT", "10.0"))
        
        # Create Redis connection with password if available
        redis_params = {
            "host": cache_host,
            "port": redis_port,
            "db": redis_db,
            "socket_timeout": socket_timeout,  # Increased from 5.0 to allow more time for operations
            "socket_connect_timeout": socket_connect_timeout,  # Increased from 5.0 to allow more time for connection
            "socket_keepalive": True,  # Enable TCP keepalive
            "socket_keepalive_options": {
                # TCP_KEEPIDLE: time before sending keepalive probes
                socket.TCP_KEEPIDLE: 30,  # Reduced from 60
                # TCP_KEEPINTVL: time between keepalive probes
                socket.TCP_KEEPINTVL: 10,  # Reduced from 30
                # TCP_KEEPCNT: number of keepalive probes
                socket.TCP_KEEPCNT: 3  # Reduced from 5
            },
            "retry_on_timeout": True,
            "health_check_interval": 10,  # Reduced from 15 to check more frequently
            "max_connections": 10,  # Limit the number of connections
            "decode_responses": False,  # Don't decode responses automatically
            "client_name": f"worker-{os.getpid()}"  # Add client name for better debugging
        }
        
        if redis_password:
            redis_params["password"] = redis_password
            
        # Create a Redis connection pool
        pool = redis.ConnectionPool(**redis_params)
        self.cache = redis.Redis(connection_pool=pool)
        
        # Test the connection
        self.cache.ping()

    def check_connection(self):
        """Check connection to database and Redis cache"""
        start_time = datetime.now()
        timeout = timedelta(minutes=1)
        
        while datetime.now() - start_time < timeout:
            redis_ok = False
            db_ok = False
            
            # Check Redis connection
            try:
                self.cache.ping()
                redis_ok = True
            except (redis.exceptions.ConnectionError, redis.exceptions.ResponseError, redis.exceptions.TimeoutError) as e:
                print(f"Redis connection error during check: {e}", flush=True)
                print("Attempting to reconnect to Redis...", flush=True)
                try:
                    # Use the same host as the initial connection
                    cache_host = os.environ.get("REDIS_HOST", "cache")
                    self._connect_to_redis(cache_host)
                    redis_ok = True
                    print("Successfully reconnected to Redis", flush=True)
                except Exception as e:
                    print(f"Failed to reconnect to Redis: {e}", flush=True)
                    raise
            
            # Check database connection
            try:
                with self.db.cursor() as cursor:
                    cursor.execute("SELECT 1")
                db_ok = True
            except psycopg2.OperationalError as e:
                print(f"Database connection error during check: {e}", flush=True)
                print("Attempting to reconnect to database...", flush=True)
                try:
                    # Use the same host as the initial connection
                    db_host = os.environ.get("DB_HOST", "db")
                    self._connect_to_db(db_host)
                    db_ok = True
                    print("Successfully reconnected to database", flush=True)
                except Exception as e:
                    print(f"Failed to reconnect to database: {e}", flush=True)
                    raise
            except Exception as e:
                print(f"Unexpected error during connection check: {e}", flush=True)
                raise
            
            if redis_ok and db_ok:
                return True
            
            time.sleep(2)  # Short sleep before retry
        
        # If we get here, we've timed out
        error_msg = "Failed to establish connections within 1 minute:\n"
        if not redis_ok:
            error_msg += "Redis connection failed\n"
        if not db_ok:
            error_msg += "Database connection failed"
        print(error_msg, flush=True)
        return False

    def get_gemini_key(self):
        """Get the next available Gemini API key."""
        try:
            return self.gemini_pool.get_next_key()
        except ValueError as e:
            print(f"Error getting Gemini API key: {e}", flush=True)
            return None
