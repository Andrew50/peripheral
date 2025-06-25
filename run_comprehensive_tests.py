#!/usr/bin/env python3
"""
Comprehensive Test Runner for Strategy System
Executes all complex strategy tests to validate AST parser, data requirements, and server functionality
"""

import asyncio
import sys
import os
import time
from datetime import datetime

# Add services/worker to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'services', 'worker', 'tests'))

async def run_all_tests():
    """Run comprehensive test suite"""
    
    print("🧪" + "="*90)
    print("🧪 COMPREHENSIVE STRATEGY SYSTEM TEST SUITE")
    print("🧪" + "="*90)
    print(f"🧪 Started at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print()
    
    test_results = {}
    total_start_time = time.time()
    
    # Test 1: Complex Strategy AST Analysis and Execution
    print("\n" + "🔬" + "="*89)
    print("🔬 TEST SUITE 1: COMPLEX STRATEGY ANALYSIS AND EXECUTION")
    print("🔬" + "="*89)
    
    try:
        from test_complex_strategies import ComplexStrategyTester
        
        tester = ComplexStrategyTester()
        strategy_results = await tester.test_all_scenarios()
        test_results['complex_strategies'] = strategy_results
        
        # Print detailed analysis
        print(f"\n📊 COMPLEX STRATEGY TEST ANALYSIS:")
        successful_strategies = [name for name, result in strategy_results.items() if result.get('success')]
        
        print(f"   ✅ Successful Strategies: {len(successful_strategies)}/{len(strategy_results)}")
        for name in successful_strategies:
            instances = len(strategy_results[name]['execution'].get('instances', []))
            complexity = strategy_results[name]['ast_analysis'].get('strategy_complexity', 'unknown')
            print(f"      • {name}: {instances} instances, complexity: {complexity}")
        
        failed_strategies = [name for name, result in strategy_results.items() if not result.get('success')]
        if failed_strategies:
            print(f"   ❌ Failed Strategies: {len(failed_strategies)}")
            for name in failed_strategies:
                error = strategy_results[name].get('error', 'Unknown error')
                print(f"      • {name}: {error}")
        
    except ImportError as e:
        print(f"❌ Could not import complex strategy tester: {e}")
        test_results['complex_strategies'] = {'success': False, 'error': str(e)}
    except Exception as e:
        print(f"❌ Error in complex strategy testing: {e}")
        test_results['complex_strategies'] = {'success': False, 'error': str(e)}
    
    # Test 2: Server Integration Tests
    print("\n" + "🔗" + "="*89)
    print("🔗 TEST SUITE 2: SERVER INTEGRATION AND QUEUE PROCESSING")  
    print("🔗" + "="*89)
    
    try:
        from test_server_integration import ServerIntegrationTester
        
        integration_tester = ServerIntegrationTester()
        integration_results = await integration_tester.test_strategy_queue_integration()
        test_results['server_integration'] = integration_results
        
        # Print integration analysis
        print(f"\n📊 SERVER INTEGRATION TEST ANALYSIS:")
        successful_integrations = [name for name, result in integration_results.items() if result.get('success')]
        
        print(f"   ✅ Successful Integrations: {len(successful_integrations)}/{len(integration_results)}")
        for name in successful_integrations:
            task_id = integration_results[name]['queue_submission'].get('task_id', 'unknown')
            instances = len(integration_results[name]['final_result'].get('instances', []))
            print(f"      • {name}: Task {task_id}, {instances} instances")
        
        failed_integrations = [name for name, result in integration_results.items() if not result.get('success')]
        if failed_integrations:
            print(f"   ❌ Failed Integrations: {len(failed_integrations)}")
            for name in failed_integrations:
                error = integration_results[name].get('error', 'Unknown error')
                print(f"      • {name}: {error}")
        
    except ImportError as e:
        print(f"❌ Could not import server integration tester: {e}")
        test_results['server_integration'] = {'success': False, 'error': str(e)}
    except Exception as e:
        print(f"❌ Error in server integration testing: {e}")
        test_results['server_integration'] = {'success': False, 'error': str(e)}
    
    # Test 3: AST Parser and Data Requirements Analysis
    print("\n" + "⚙️" + "="*89)
    print("⚙️ TEST SUITE 3: AST PARSER AND DATA REQUIREMENTS VALIDATION")
    print("⚙️" + "="*89)
    
    ast_test_results = await run_ast_parser_tests()
    test_results['ast_parser'] = ast_test_results
    
    # Test 4: Strategy Validation and Security
    print("\n" + "🔒" + "="*89)
    print("🔒 TEST SUITE 4: STRATEGY VALIDATION AND SECURITY")
    print("🔒" + "="*89)
    
    validation_test_results = await run_validation_tests()
    test_results['validation'] = validation_test_results
    
    # Test 5: Multi-Timeframe and Edge Cases
    print("\n" + "📈" + "="*89)
    print("📈 TEST SUITE 5: MULTI-TIMEFRAME AND EDGE CASES")
    print("📈" + "="*89)
    
    edge_case_results = await run_edge_case_tests()
    test_results['edge_cases'] = edge_case_results
    
    # Final Summary
    total_execution_time = time.time() - total_start_time
    print_final_summary(test_results, total_execution_time)
    
    return test_results

async def run_ast_parser_tests():
    """Test AST parser and data requirements analysis"""
    
    print("🔍 Testing AST Parser and Data Requirements Analysis...")
    
    try:
        sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'services', 'worker', 'src'))
        from strategy_data_analyzer import StrategyDataAnalyzer
        from validator import SecurityValidator
        
        analyzer = StrategyDataAnalyzer()
        validator = SecurityValidator()
        
        test_strategies = [
            {
                'name': 'Gold Gap Analysis',
                'code': '''
def strategy(data):
    for i in range(data.shape[0]):
        ticker = data[i, 0]
        if ticker in ['GLD', 'GOLD']:
            gap = (float(data[i, 2]) - float(data[i-1, 5])) / float(data[i-1, 5]) * 100
            if gap > 3.0:
                return [{'ticker': ticker, 'signal': True}]
    return []
''',
                'expected_features': ['gap_analysis', 'symbol_filtering']
            },
            {
                'name': 'Sector Performance',
                'code': '''
def strategy(data):
    instances = []
    for i in range(data.shape[0]):
        sector = data[i, 11] if len(data[i]) > 11 else 'Unknown'
        if sector == 'Technology':
            return_pct = (float(data[i, 5]) - float(data[i-20, 5])) / float(data[i-20, 5]) * 100
            if return_pct > 10:
                instances.append({'ticker': data[i, 0], 'signal': True})
    return instances
''',
                'expected_features': ['sector_analysis', 'fundamental_data']
            },
            {
                'name': 'Technical Indicators',
                'code': '''
def strategy(data):
    instances = []
    for i in range(26, data.shape[0]):
        ticker = data[i, 0]
        high = float(data[i, 3])
        low = float(data[i, 4])
        close = float(data[i, 5])
        adr = sum([float(data[j, 3]) - float(data[j, 4]) for j in range(i-13, i+1)]) / 14
        daily_return = (close - float(data[i-1, 5])) / float(data[i-1, 5]) * 100
        if daily_return > adr * 3:
            instances.append({'ticker': ticker, 'signal': True})
    return instances
''',
                'expected_features': ['technical_indicators', 'adr_calculation']
            }
        ]
        
        ast_results = {}
        
        for strategy in test_strategies:
            print(f"\n   🔍 Analyzing: {strategy['name']}")
            
            # Test AST analysis
            analysis = analyzer.analyze_data_requirements(strategy['code'], mode='backtest')
            
            # Test validation
            is_valid = validator.validate_code(strategy['code'])
            
            ast_results[strategy['name']] = {
                'ast_analysis': analysis,
                'validation_passed': is_valid,
                'complexity': analysis.get('strategy_complexity', 'unknown'),
                'loading_strategy': analysis.get('loading_strategy', 'unknown'),
                'required_columns': analysis.get('data_requirements', {}).get('columns', []),
                'success': is_valid and analysis.get('strategy_complexity') is not None
            }
            
            complexity = analysis.get('strategy_complexity', 'unknown')
            loading = analysis.get('loading_strategy', 'unknown')
            columns = len(analysis.get('data_requirements', {}).get('columns', []))
            
            print(f"      ✅ Complexity: {complexity}")
            print(f"      📊 Loading Strategy: {loading}")
            print(f"      📋 Required Columns: {columns}")
            print(f"      🔒 Validation: {'PASS' if is_valid else 'FAIL'}")
        
        success_count = len([r for r in ast_results.values() if r['success']])
        print(f"\n   📊 AST Parser Tests: {success_count}/{len(ast_results)} passed")
        
        return {
            'success': success_count == len(ast_results),
            'results': ast_results,
            'passed': success_count,
            'total': len(ast_results)
        }
        
    except Exception as e:
        print(f"   ❌ AST Parser test failed: {e}")
        return {'success': False, 'error': str(e)}

async def run_validation_tests():
    """Test strategy validation and security"""
    
    print("🔒 Testing Strategy Validation and Security...")
    
    try:
        sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'services', 'worker', 'src'))
        from validator import SecurityValidator
        
        validator = SecurityValidator()
        
        test_cases = [
            {
                'name': 'Valid Strategy',
                'code': 'def strategy(data): return [{"ticker": "AAPL", "signal": True}]',
                'should_pass': True
            },
            {
                'name': 'Import Restriction',
                'code': 'import os\ndef strategy(data): return []',
                'should_pass': False
            },
            {
                'name': 'File Access Restriction', 
                'code': 'def strategy(data): open("file.txt"); return []',
                'should_pass': False
            },
            {
                'name': 'Valid Data Access',
                'code': 'def strategy(data): ticker = data[0, 0]; return [{"ticker": ticker}]',
                'should_pass': True
            }
        ]
        
        validation_results = {}
        
        for test_case in test_cases:
            print(f"\n   🔒 Testing: {test_case['name']}")
            
            try:
                is_valid = validator.validate_code(test_case['code'])
                expected = test_case['should_pass']
                passed = (is_valid == expected)
                
                validation_results[test_case['name']] = {
                    'validation_result': is_valid,
                    'expected': expected,
                    'test_passed': passed
                }
                
                status = "✅ PASS" if passed else "❌ FAIL"
                print(f"      {status} Validation: {is_valid}, Expected: {expected}")
                
            except Exception as e:
                validation_results[test_case['name']] = {
                    'validation_result': False,
                    'expected': test_case['should_pass'],
                    'test_passed': not test_case['should_pass'],  # Exceptions are expected for invalid code
                    'error': str(e)
                }
                print(f"      ✅ Exception (expected for invalid code): {e}")
        
        success_count = len([r for r in validation_results.values() if r['test_passed']])
        print(f"\n   📊 Validation Tests: {success_count}/{len(validation_results)} passed")
        
        return {
            'success': success_count == len(validation_results),
            'results': validation_results,
            'passed': success_count,
            'total': len(validation_results)
        }
        
    except Exception as e:
        print(f"   ❌ Validation test failed: {e}")
        return {'success': False, 'error': str(e)}

async def run_edge_case_tests():
    """Test edge cases and multi-timeframe scenarios"""
    
    print("📈 Testing Edge Cases and Multi-Timeframe Scenarios...")
    
    try:
        sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'services', 'worker', 'src'))
        from strategy_data_analyzer import StrategyDataAnalyzer
        
        analyzer = StrategyDataAnalyzer()
        
        edge_cases = [
            {
                'name': 'Empty Strategy',
                'code': 'def strategy(data): return []',
                'mode': 'screener'
            },
            {
                'name': 'Large Dataset Strategy',
                'code': '''
def strategy(data):
    instances = []
    for i in range(min(10000, data.shape[0])):
        if float(data[i, 5]) > 100:
            instances.append({"ticker": data[i, 0], "signal": True})
    return instances
''',
                'mode': 'backtest'
            },
            {
                'name': 'Multi-Timeframe Strategy',
                'code': '''
def strategy(data):
    daily_data = data[data[:, 1] % 86400 == 0]  # Filter daily data
    minute_data = data[data[:, 1] % 60 == 0]    # Filter minute data
    return [{"ticker": "AAPL", "signal": True}]
''',
                'mode': 'alert'
            },
            {
                'name': 'Complex Mathematical Strategy',
                'code': '''
def strategy(data):
    instances = []
    for i in range(50, data.shape[0]):
        prices = [float(data[j, 5]) for j in range(i-49, i+1)]
        mean = sum(prices) / len(prices)
        variance = sum((p - mean) ** 2 for p in prices) / len(prices)
        std = variance ** 0.5
        
        if abs(float(data[i, 5]) - mean) > 2 * std:
            instances.append({"ticker": data[i, 0], "signal": True, "z_score": abs(float(data[i, 5]) - mean) / std})
    return instances
''',
                'mode': 'backtest'
            }
        ]
        
        edge_results = {}
        
        for edge_case in edge_cases:
            print(f"\n   📈 Testing: {edge_case['name']}")
            
            try:
                analysis = analyzer.analyze_data_requirements(edge_case['code'], mode=edge_case['mode'])
                
                edge_results[edge_case['name']] = {
                    'analysis_completed': True,
                    'complexity': analysis.get('strategy_complexity', 'unknown'),
                    'mode_optimization': analysis.get('data_requirements', {}).get('mode_optimization', 'unknown'),
                    'estimated_rows': analysis.get('data_requirements', {}).get('estimated_rows', 0),
                    'success': True
                }
                
                complexity = analysis.get('strategy_complexity', 'unknown')
                optimization = analysis.get('data_requirements', {}).get('mode_optimization', 'unknown')
                print(f"      ✅ Complexity: {complexity}")
                print(f"      🎯 Mode Optimization: {optimization}")
                
            except Exception as e:
                edge_results[edge_case['name']] = {
                    'analysis_completed': False,
                    'error': str(e),
                    'success': False
                }
                print(f"      ❌ Analysis failed: {e}")
        
        success_count = len([r for r in edge_results.values() if r['success']])
        print(f"\n   📊 Edge Case Tests: {success_count}/{len(edge_results)} passed")
        
        return {
            'success': success_count == len(edge_results),
            'results': edge_results,
            'passed': success_count,
            'total': len(edge_results)
        }
        
    except Exception as e:
        print(f"   ❌ Edge case test failed: {e}")
        return {'success': False, 'error': str(e)}

def print_final_summary(test_results, execution_time):
    """Print comprehensive test summary"""
    
    print("\n" + "🏆" + "="*89)
    print("🏆 COMPREHENSIVE TEST SUITE SUMMARY")
    print("🏆" + "="*89)
    
    total_tests = 0
    total_passed = 0
    
    for suite_name, results in test_results.items():
        if isinstance(results, dict):
            if 'passed' in results and 'total' in results:
                # Simple pass/total format
                passed = results['passed']
                total = results['total']
                total_tests += total
                total_passed += passed
                
                status = "✅" if results.get('success') else "❌"
                print(f"{status} {suite_name.replace('_', ' ').title()}: {passed}/{total} tests passed")
                
            elif isinstance(results, dict) and all(isinstance(v, dict) for v in results.values()):
                # Nested results format (like complex_strategies)
                passed = len([r for r in results.values() if r.get('success')])
                total = len(results)
                total_tests += total
                total_passed += passed
                
                status = "✅" if passed == total else "❌"
                print(f"{status} {suite_name.replace('_', ' ').title()}: {passed}/{total} tests passed")
            
            else:
                # Simple success/failure
                success = results.get('success', False)
                total_tests += 1
                total_passed += 1 if success else 0
                
                status = "✅" if success else "❌"
                print(f"{status} {suite_name.replace('_', ' ').title()}: {'PASSED' if success else 'FAILED'}")
    
    print("\n" + "📊" + "="*89)
    print("📊 OVERALL RESULTS")
    print("📊" + "="*89)
    
    success_rate = (total_passed / total_tests * 100) if total_tests > 0 else 0
    
    print(f"📋 Total Tests Executed: {total_tests}")
    print(f"✅ Tests Passed: {total_passed}")
    print(f"❌ Tests Failed: {total_tests - total_passed}")
    print(f"📈 Success Rate: {success_rate:.1f}%")
    print(f"⏱️ Total Execution Time: {execution_time:.2f} seconds")
    print(f"🕐 Completed at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    
    # Recommendations
    print("\n" + "💡" + "="*89)
    print("💡 RECOMMENDATIONS")
    print("💡" + "="*89)
    
    if success_rate >= 90:
        print("🎉 Excellent! The strategy system is performing very well.")
        print("   All major components are functioning correctly.")
    elif success_rate >= 75:
        print("👍 Good performance with some areas for improvement.")
        print("   Consider investigating failed test cases for optimization.")
    elif success_rate >= 50:
        print("⚠️ Moderate performance - significant improvements needed.")
        print("   Focus on core functionality and error handling.")
    else:
        print("🚨 Poor performance - major issues detected.")
        print("   Comprehensive debugging and fixes required.")
    
    print("\n" + "🧪" + "="*89)
    print("🧪 TEST SUITE COMPLETED")
    print("🧪" + "="*89)

if __name__ == "__main__":
    try:
        results = asyncio.run(run_all_tests())
    except KeyboardInterrupt:
        print("\n🛑 Test execution interrupted by user")
    except Exception as e:
        print(f"\n💥 Test execution failed with error: {e}")
        import traceback
        traceback.print_exc()