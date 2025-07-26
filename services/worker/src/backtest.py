async def backtest(ctx: Context, task_id: str = None, symbols: List[str] = None, 
                            start_date: str = None, end_date: str = None, 
                            securities: List[str] = None, strategy_id: str = None, version: int = None, **kwargs) -> Dict[str, Any]:
    """Execute backtest task using new accessor strategy engine"""
    if not strategy_id:
        raise ValueError("strategy_id is required")
        
    if task_id:
        ctx._publish_progress(task_id, "Loading strategy")
    
    strategy_code, version = ctx.strategy_crud.fetch_strategy_code(strategy_id, version)
    strategy_code, version = fetch_strategy_code(ctx, strategy_id, version)
    
    # Handle symbols and securities filtering
    symbols_input = symbols or []
    securities_filter = securities or []
    
    # Determine target symbols (strategies will fetch their own data via accessors)
    if securities_filter:
        target_symbols = securities_filter
        logger.debug(f"Using securities filter as target symbols: {len(target_symbols)} symbols")
    elif symbols_input:
        target_symbols = symbols_input
        logger.debug(f"Using provided symbols: {len(target_symbols)} symbols")
    else:
        target_symbols = []  # Let strategy determine its own symbols
        logger.debug("No symbols specified - strategy will determine requirements")
    
    if task_id:
        ctx._publish_progress(task_id, "symbols", f"Prepared {len(target_symbols)} symbols for analysis", 
                                {"symbol_count": len(target_symbols)})
    
    #logger.info(f"Starting backtest for {len(target_symbols)} symbols (strategy_id: {strategy_id})")
    
    if task_id:
        ctx._publish_progress(task_id, "preparation", "Preparing date ranges and execution parameters...")
    
    # Parse and validate dates
    parsed_start_date = None
    parsed_end_date = None
    
    if start_date:
        try:
            # Parse as YYYY-MM-DD format only
            parsed_start_date = datetime.strptime(start_date, '%Y-%m-%d')
        except (ValueError, TypeError) as e:
            logger.warning(f"Invalid start_date format '{start_date}': {e}. Expected YYYY-MM-DD format. Using default.")
            parsed_start_date = None
    
    if end_date:
        try:
            # Parse as YYYY-MM-DD format only
            parsed_end_date = datetime.strptime(end_date, '%Y-%m-%d')
        except (ValueError, TypeError) as e:
            logger.warning(f"Invalid end_date format '{end_date}': {e}. Expected YYYY-MM-DD format. Using default.")
            parsed_end_date = None
    
    # Set defaults if parsing failed or dates not provided
    if not parsed_start_date:
        parsed_start_date = datetime.now() - timedelta(days=365)  # Default 1 year
        #logger.info(f"Using default start_date: {parsed_start_date.date()}")
    
    if not parsed_end_date:
        parsed_end_date = datetime.now()
        #logger.info(f"Using default end_date: {parsed_end_date.date()}")
    
    if parsed_start_date > parsed_end_date:
        raise ValueError(f"start_date ({parsed_start_date.date()}) must be before end_date ({parsed_end_date.date()})")
    
    # Log the final date range
    #logger.info(f"Backtest date range: {parsed_start_date.date()} to {parsed_end_date.date()}")
    
    if task_id:
        ctx._publish_progress(task_id, f"Executing backtest: {parsed_start_date.date()} to {parsed_end_date.date()}", 
                                {"start_date": parsed_start_date.isoformat(), "end_date": parsed_end_date.isoformat(), 
                                "symbol_count": len(target_symbols)})
    
    # Execute using accessor strategy engine
    start_time = time.time()
    # Execute strategy with accessor context
    instances, strategy_prints, strategy_plots, response_images, error = await _execute_strategy(
        strategy_code, 
        execution_mode='backtest',
        symbols=symbols,
        start_date=start_date,
        end_date=end_date,
        max_instances=max_instances,
        strategy_id=strategy_id,
        version=version
    )
    if error: 
        raise error
    
    execution_time = (time.time() - start_time) * 1000
    date_range = [start_date.strftime('%Y-%m-%d'), end_date.strftime('%Y-%m-%d')]
    
    result = {
        'success': True,
        'version': version,
        'instances': instances,
        'symbols_processed': len(symbols),
        'strategy_prints': strategy_prints,
        'strategy_plots': strategy_plots,
        'response_images': response_images,
        'instance_limit_reached': TrackedList.is_limit_reached(),
        'summary': {
            'total_instances': len(instances),
            'symbols_processed': len(symbols),
            'date_range': date_range,
        },
    }
 
    return result