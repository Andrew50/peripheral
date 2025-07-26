async def screen(self, task_id: str = None, universe: List[str] = None, 
                            limit: int = 100, strategy_ids: List[str] = None, **kwargs) -> Dict[str, Any]:
    """Execute screening task using new accessor strategy engine"""
    if not strategy_ids:
        raise ValueError("strategy_ids is required")
        
    strategy_codes = self.strategy_crud.fetch_multiple_strategy_codes(strategy_ids)
    #logger.info(f"Fetched {len(strategy_codes)} strategy codes from database")
    
    # For now, use the first strategy code for screening
    # TODO: Implement multi-strategy screening in the future
    if strategy_codes:
        strategy_code = list(strategy_codes.values())[0]
    else:
        raise ValueError("No valid strategy codes found for provided strategy_ids")
    
    
    # Use provided universe or let strategy determine requirements
    target_universe = universe or []
    #logger.info(f"Starting screening for {len(target_universe)} symbols, limit {limit} (strategy_ids: {strategy_ids})")
    
    # Execute using accessor strategy engine
    start_time = time.time()
        
        try:
            
            # Set execution context for data accessors with screening optimizations
            set_execution_context(
                mode='screening',
                symbols=universe
            )
            
            # Execute strategy with accessor context
            instances, _, _, _, error = await _execute_strategy(
                strategy_code, 
                execution_mode='screening',
                max_instances=max_instances
            )
            if error: 
                raise error
            # Rank and limit results
            ranked_results = _rank_screening_results(instances, limit)
            
            execution_time = (time.time() - start_time) * 1000
            
            result = {
                'success': True,
                'execution_mode': 'screening',
                'ranked_results': ranked_results,
                'universe_size': len(universe),
                'results_returned': len(ranked_results),
                'instance_limit_reached': TrackedList.is_limit_reached(),
                'max_instances_configured': max_instances,
                'execution_time_ms': int(execution_time),  # Convert to integer for Go compatibility
                'optimization_enabled': True,
                'data_strategy': 'minimal_recent'
            }
            
            #logger.info(f"✅ Screening completed: {len(ranked_results)} results, {execution_time:.1f}ms")
            logger.debug(f"   📈 Performance: {len(ranked_results)/execution_time*1000:.1f} results/second")
            return result
    
    #logger.info(f"Screening completed: {len(result.get('ranked_results', []))} results found")
    return result

def _rank_screening_results(instances: List[Dict], limit: int) -> List[Dict]:
    """Rank screening results by score or other criteria and convert to WorkerRankedResult format"""
    
    # Sort by score if available, otherwise by timestamp descending (most recent first)
    def sort_key(instance):
        if 'score' in instance:
            return instance['score']
        else:
            # Use timestamp for sorting if no score - more recent = higher priority
            return instance.get('timestamp', 0)
    
    sorted_instances = sorted(instances, key=sort_key, reverse=True)
    
    # Limit results
    limited_instances = sorted_instances[:limit]
    
    # Convert to WorkerRankedResult format expected by Go backend
    ranked_results = []
    for instance in limited_instances:
        # Convert instance to WorkerRankedResult format
        ranked_result = {
            'symbol': instance.get('ticker', ''),  # Convert ticker to symbol
            'score': float(instance.get('score', 0.0)),
            'current_price': float(instance.get('entry_price', instance.get('close', instance.get('price', 0.0)))),
            'sector': instance.get('sector', ''),
            'data': instance  # Include the full instance data
        }
        ranked_results.append(ranked_result)
    
    return ranked_result