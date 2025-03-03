import time, psycopg2, redis, os, sys, socket
from datetime import datetime, timedelta


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
        
        # Create Redis connection with password if available
        redis_params = {
            "host": cache_host,
            "port": redis_port,
            "db": redis_db,
            "socket_timeout": 5.0,  # Reduced from 10.0 to fail faster
            "socket_connect_timeout": 5.0,  # Reduced from 10.0 to fail faster
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
        """Check both database and Redis connections."""
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
            
        return redis_ok and db_ok
