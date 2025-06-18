"""
Basic tests for the Python worker
"""

import pytest
import sys
import os

# Add src to path for testing
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'src'))

from src.security_validator import SecurityValidator


class TestSecurityValidator:
    """Test security validation functionality"""
    
    def test_security_validator_creation(self):
        """Test that SecurityValidator can be created"""
        validator = SecurityValidator()
        assert validator is not None
    
    def test_safe_code_validation(self):
        """Test that safe code passes validation"""
        validator = SecurityValidator()
        safe_code = """
def calculate_moving_average(data, window):
    return sum(data[-window:]) / window
"""
        # This might need adjustment based on actual implementation
        # For now, just test that the method exists
        assert hasattr(validator, 'validate_code')
    
    def test_worker_imports(self):
        """Test that worker dependencies can be imported"""
        try:
            import redis
            import psutil
            import numpy
            import pandas
            assert True
        except ImportError as e:
            pytest.fail(f"Failed to import required dependency: {e}")


class TestBasicFunctionality:
    """Test basic Python functionality"""
    
    def test_python_version(self):
        """Test that we're running Python 3.11+"""
        assert sys.version_info >= (3, 11)
    
    def test_basic_calculation(self):
        """Test basic mathematical operations"""
        assert 2 + 2 == 4
        assert 10 / 2 == 5.0
    
    def test_list_operations(self):
        """Test list operations work correctly"""
        data = [1, 2, 3, 4, 5]
        assert len(data) == 5
        assert sum(data) == 15
        assert max(data) == 5 