"""
Custom exception classes for the worker system.
Provides semantic clarity for different types of failures.
"""

class WorkerError(Exception):
    """Base exception for all worker-related errors"""
    pass

class StrategyExecutionError(WorkerError):
    """Raised when strategy execution fails"""
    pass

class StrategyValidationError(WorkerError):
    """Raised when strategy validation fails"""
    pass

class TaskExecutionError(WorkerError):
    """Raised when task execution fails"""
    pass

class ModelGenerationError(WorkerError):
    """Raised when AI model generation fails"""
    pass 