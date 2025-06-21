"""
Worker HTTP Server
Provides HTTP endpoints for the three strategy worker functions:
- run_backtest
- run_screener  
- run_alert
"""

import asyncio
import json
import logging
from datetime import datetime
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import threading

try:
    from .strategy_worker import run_backtest, run_screener, run_alert
except ImportError:
    from strategy_worker import run_backtest, run_screener, run_alert

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class WorkerRequestHandler(BaseHTTPRequestHandler):
    """HTTP request handler for worker functions"""
    
    def do_POST(self):
        """Handle POST requests"""
        try:
            # Parse URL
            parsed_url = urlparse(self.path)
            
            if parsed_url.path == '/execute':
                self._handle_execute()
            else:
                self._send_error(404, "Not Found")
                
        except Exception as e:
            logger.error(f"Error handling POST request: {e}")
            self._send_error(500, f"Internal Server Error: {str(e)}")
    
    def do_GET(self):
        """Handle GET requests"""
        try:
            parsed_url = urlparse(self.path)
            
            if parsed_url.path == '/health':
                self._handle_health()
            else:
                self._send_error(404, "Not Found")
                
        except Exception as e:
            logger.error(f"Error handling GET request: {e}")
            self._send_error(500, f"Internal Server Error: {str(e)}")
    
    def _handle_execute(self):
        """Handle strategy execution requests"""
        try:
            # Read request body
            content_length = int(self.headers.get('Content-Length', 0))
            if content_length == 0:
                self._send_error(400, "Empty request body")
                return
            
            body = self.rfile.read(content_length)
            request_data = json.loads(body.decode('utf-8'))
            
            # Validate request
            if 'function' not in request_data or 'strategy_id' not in request_data:
                self._send_error(400, "Missing required fields: function, strategy_id")
                return
            
            function_name = request_data['function']
            strategy_id = request_data['strategy_id']
            
            logger.info(f"Executing {function_name} for strategy {strategy_id}")
            
            # Route to appropriate function
            if function_name == 'run_backtest':
                result = asyncio.run(self._run_backtest(strategy_id, request_data))
            elif function_name == 'run_screener':
                result = asyncio.run(self._run_screener(strategy_id, request_data))
            elif function_name == 'run_alert':
                result = asyncio.run(self._run_alert(strategy_id, request_data))
            else:
                self._send_error(400, f"Unknown function: {function_name}")
                return
            
            # Send response
            self._send_json_response(result)
            
        except json.JSONDecodeError as e:
            self._send_error(400, f"Invalid JSON: {str(e)}")
        except Exception as e:
            logger.error(f"Error executing function: {e}")
            self._send_error(500, f"Execution error: {str(e)}")
    
    def _handle_health(self):
        """Handle health check requests"""
        health_status = {
            "status": "healthy",
            "timestamp": datetime.utcnow().isoformat(),
            "functions": ["run_backtest", "run_screener", "run_alert"]
        }
        self._send_json_response(health_status)
    
    async def _run_backtest(self, strategy_id: int, request_data: dict):
        """Execute backtest function"""
        kwargs = {}
        
        # Extract optional parameters
        if 'start_date' in request_data:
            kwargs['start_date'] = request_data['start_date']
        if 'end_date' in request_data:
            kwargs['end_date'] = request_data['end_date']
        if 'symbols' in request_data:
            kwargs['symbols'] = request_data['symbols']
        
        return await run_backtest(strategy_id, **kwargs)
    
    async def _run_screener(self, strategy_id: int, request_data: dict):
        """Execute screener function"""
        kwargs = {}
        
        # Extract optional parameters
        if 'universe' in request_data:
            kwargs['universe'] = request_data['universe']
        if 'limit' in request_data:
            kwargs['limit'] = request_data['limit']
        
        return await run_screener(strategy_id, **kwargs)
    
    async def _run_alert(self, strategy_id: int, request_data: dict):
        """Execute alert function"""
        kwargs = {}
        
        # Extract optional parameters
        if 'symbols' in request_data:
            kwargs['symbols'] = request_data['symbols']
        if 'alert_threshold' in request_data:
            kwargs['alert_threshold'] = request_data['alert_threshold']
        
        return await run_alert(strategy_id, **kwargs)
    
    def _send_json_response(self, data: dict):
        """Send JSON response"""
        response_json = json.dumps(data, indent=2)
        
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(response_json)))
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'POST, GET, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type')
        self.end_headers()
        
        self.wfile.write(response_json.encode('utf-8'))
    
    def _send_error(self, code: int, message: str):
        """Send error response"""
        error_response = {
            "success": False,
            "error_message": message,
            "timestamp": datetime.utcnow().isoformat()
        }
        
        response_json = json.dumps(error_response, indent=2)
        
        self.send_response(code)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(response_json)))
        self.send_header('Access-Control-Allow-Origin', '*')
        self.end_headers()
        
        self.wfile.write(response_json.encode('utf-8'))
    
    def do_OPTIONS(self):
        """Handle CORS preflight requests"""
        self.send_response(200)
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'POST, GET, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type')
        self.end_headers()
    
    def log_message(self, format, *args):
        """Override to use Python logging instead of stderr"""
        logger.info(f"{self.address_string()} - {format % args}")


class WorkerServer:
    """HTTP server for worker functions"""
    
    def __init__(self, host='localhost', port=8080):
        self.host = host
        self.port = port
        self.server = None
        self.server_thread = None
    
    def start(self):
        """Start the server"""
        self.server = HTTPServer((self.host, self.port), WorkerRequestHandler)
        logger.info(f"Starting worker server on {self.host}:{self.port}")
        
        # Run server in a separate thread
        self.server_thread = threading.Thread(target=self.server.serve_forever)
        self.server_thread.daemon = True
        self.server_thread.start()
        
        logger.info(f"Worker server started successfully")
        return self
    
    def stop(self):
        """Stop the server"""
        if self.server:
            logger.info("Stopping worker server...")
            self.server.shutdown()
            self.server.server_close()
            
            if self.server_thread:
                self.server_thread.join(timeout=5)
            
            logger.info("Worker server stopped")
    
    def wait(self):
        """Wait for the server thread to finish"""
        if self.server_thread:
            try:
                self.server_thread.join()
            except KeyboardInterrupt:
                logger.info("Received interrupt signal")
                self.stop()


def main():
    """Main function to start the worker server"""
    import argparse
    
    parser = argparse.ArgumentParser(description='Strategy Worker HTTP Server')
    parser.add_argument('--host', default='localhost', help='Server host (default: localhost)')
    parser.add_argument('--port', type=int, default=8080, help='Server port (default: 8080)')
    
    args = parser.parse_args()
    
    # Start server
    server = WorkerServer(host=args.host, port=args.port)
    server.start()
    
    try:
        logger.info("Worker server is running. Press Ctrl+C to stop.")
        server.wait()
    except KeyboardInterrupt:
        logger.info("Shutting down worker server...")
        server.stop()


if __name__ == '__main__':
    main() 