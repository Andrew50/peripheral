

async def alert(ctx: Context, task_id: str = None, symbols: List[str] = None, 
                    strategy_id: str = None, **kwargs) -> Dict[str, Any]:
    """Execute alert task using new accessor strategy engine"""
    if not strategy_id:
        raise ValueError("strategy_id is required")
        
    # _fetch_strategy_code returns a tuple (python_code, version). Unpack it properly.
    strategy_code, version = fetch_strategy_code(ctx, strategy_id)
    #logger.info(
        #f"Fetched strategy code (version {version}) from database for strategy_id: {strategy_id}"
    #)
    
    # Handle universe parameter (new) or legacy symbols parameter
    universe = kwargs.get('universe')
    if universe is not None:
        target_symbols = universe
        #logger.info(f"Starting alert with provided universe: {len(target_symbols)} symbols (strategy_id: {strategy_id})")
    elif symbols is not None:
        target_symbols = symbols
        #logger.info(f"Starting alert with provided symbols: {len(target_symbols)} symbols (strategy_id: {strategy_id})")
    else:
        # No universe specified - use default alert universe
        target_symbols = _get_default_alert_universe()
        #logger.info(f"Starting alert with default universe: {len(target_symbols)} symbols (strategy_id: {strategy_id})")
    
    # Forward both the strategy code and its version to the strategy engine.
    
        strategy_id = kwargs.get('strategy_id', 'unknown')
        user_id = kwargs.get('user_id', 'unknown')
        #logger.info(f"🔔 Starting strategy alert execution - ID: {strategy_id}, User: {user_id}")
        #logger.info(f"🔔 Alert universe: {len(symbols)} symbols")
        
        # Log first few symbols for debugging
        if len(symbols) > 0:
            sample_symbols = symbols[:5]  # Show first 5 symbols
            #logger.info(f"🔔 Sample symbols: {sample_symbols}")
        else:
            logger.warning(f"🔔 No symbols provided for alert execution")
        
        start_time = time.time()
        
        try:
            # Log strategy parameters
            #logger.info(f"🔔 Alert execution parameters: max_instances={max_instances}")
            
            # Set execution context for data accessors
            #logger.info(f"🔔 Setting execution context for alert mode")
            set_execution_context(
                mode='alert',
                symbols=symbols
            )
            
            # Execute strategy with accessor context
            #logger.info(f"🔔 Executing strategy code")
            instances, strategy_prints, strategy_plots, response_images, error = await _execute_strategy(
                strategy_code, 
                execution_mode='alert',
                max_instances=max_instances,
                strategy_id=strategy_id
            )
            if error: 
                logger.error(f"🔔❌ Strategy alert execution failed: {error}")
                raise error
                
            # Log any strategy output
            #if strategy_prints:
                #logger.info(f"🔔 Strategy output:\n{strategy_prints}")
                
            # Convert instances to alerts
            #logger.info(f"🔔 Converting {len(instances)} instances to alerts")
            alerts = _convert_instances_to_alerts(instances)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'alert',
                'alerts': alerts,
                'signals': {inst['ticker']: inst for inst in instances},  # All instances are signals
                'symbols_processed': len(symbols),
                'instance_limit_reached': TrackedList.is_limit_reached(),
                'max_instances_configured': max_instances,
                'execution_time_ms': int(execution_time)  # Convert to integer for Go compatibility
            }
            
            #logger.info(f"🔔✅ Alert scan completed: {len(alerts)} alerts, {execution_time:.1f}ms")
            return result
            
        except Exception as e:
            execution_time = (time.time() - start_time) * 1000
            
            # Get detailed error information
            error_info = _get_detailed_error_info(e, strategy_code)
            detailed_error_msg = _format_detailed_error(error_info)
            
            logger.error(f"🔔❌ Alert execution failed: {e}")
            logger.error(detailed_error_msg)
            
            return {
                'success': False,
                'error': str(e),
                'error_details': error_info,
                'execution_mode': 'alert',
                'execution_time_ms': int(execution_time)

    
    #logger.info(f"Alert completed: {result.get('success', False)}")
    return result

    def _convert_instances_to_alerts(instances: List[Dict]) -> List[Dict]:
        """Convert instances to alert format for real-time mode"""
        
        alerts = []
        logger.debug(f"Converting {len(instances)} instances to alerts")
        
        for instance in instances:
            # Since all instances are signals (they met criteria), convert all to alerts
            symbol = instance['ticker']
            message = instance.get('message', f"{symbol} triggered strategy signal")
            
            alert = {
                'symbol': symbol,
                'type': 'strategy_signal',
                'message': message,
                'timestamp': dt.now().isoformat(),
                'data': instance
            }
            
            # Add priority based on score/strength
            if 'score' in instance:
                score = instance['score']
                alert['priority'] = 'high' if score > 0.8 else 'medium'
                logger.debug(f"Alert for {symbol} with score {score:.2f} - priority: {alert['priority']}")
            elif 'signal_strength' in instance:
                strength = instance['signal_strength']
                alert['priority'] = 'high' if strength > 0.8 else 'medium'
                logger.debug(f"Alert for {symbol} with signal strength {strength:.2f} - priority: {alert['priority']}")
            else:
                alert['priority'] = 'medium'
                logger.debug(f"Alert for {symbol} with default medium priority")
            
            alerts.append(alert)
            logger.debug(f"Created alert: {symbol} - {message}")
        
        return alerts