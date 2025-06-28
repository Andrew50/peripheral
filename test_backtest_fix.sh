#!/bin/bash

echo "🔧 DATABASE CONNECTION FIX - DEPLOYMENT & TEST SCRIPT"
echo "=" * 60

# Function to check if docker-compose is available
check_docker_compose() {
    if ! command -v docker-compose &> /dev/null; then
        echo "❌ docker-compose not found. Please install docker-compose."
        exit 1
    fi
    echo "✅ docker-compose found"
}

# Function to rebuild and restart services
deploy_fix() {
    echo "🚀 Deploying database connection fix..."
    
    echo "📦 Stopping existing containers..."
    docker-compose down
    
    echo "🔨 Rebuilding worker services..."
    docker-compose build worker-1 worker-2 worker-3
    
    echo "▶️ Starting services..."
    docker-compose up -d
    
    echo "⏳ Waiting for services to start..."
    sleep 10
}

# Function to test the worker
test_worker() {
    echo "🧪 Testing worker database connectivity..."
    
    # Try to run the test script in worker-1
    echo "Running database connection test..."
    docker-compose exec -T worker-1 python /app/test_db_recovery.py
    
    if [ $? -eq 0 ]; then
        echo "✅ Database connection test passed!"
        return 0
    else
        echo "❌ Database connection test failed!"
        return 1
    fi
}

# Function to monitor logs
monitor_logs() {
    echo "📝 Monitoring worker logs for 30 seconds..."
    echo "Look for:"
    echo "  ✅ 'Database connection restored' (connection recovery)"
    echo "  ✅ 'Completed backtest task' (successful backtests)"
    echo "  ❌ 'server closed the connection unexpectedly' (should not appear)"
    echo ""
    echo "Press Ctrl+C to stop monitoring"
    echo ""
    
    timeout 30 docker-compose logs -f worker-1 worker-2 worker-3 || true
}

# Function to show usage instructions
show_usage() {
    echo ""
    echo "📋 MANUAL TESTING INSTRUCTIONS"
    echo "=" * 40
    echo "1. Open your web interface"
    echo "2. Create a strategy with prompt: 'backtest a strategy where mrna gaps up 1%'"
    echo "3. The system should:"
    echo "   ✅ Successfully create the strategy"
    echo "   ✅ Successfully run the backtest (no connection errors)"
    echo "4. Check logs for connection recovery messages if needed"
    echo ""
    echo "🔍 To view logs manually:"
    echo "   docker-compose logs -f worker-1 worker-2 worker-3"
    echo ""
    echo "🧪 To run connection test manually:"
    echo "   docker-compose exec worker-1 python /app/test_db_recovery.py"
}

# Main execution
main() {
    check_docker_compose
    
    echo ""
    read -p "Deploy the database connection fix? (y/N): " -n 1 -r
    echo ""
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        deploy_fix
        
        echo ""
        read -p "Run automated database connection test? (y/N): " -n 1 -r
        echo ""
        
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            test_worker
        fi
        
        echo ""
        read -p "Monitor logs for 30 seconds? (y/N): " -n 1 -r
        echo ""
        
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            monitor_logs
        fi
        
        show_usage
    else
        echo "❌ Deployment cancelled"
        exit 1
    fi
}

# Run main function
main 