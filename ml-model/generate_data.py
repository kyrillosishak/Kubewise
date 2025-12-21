#!/usr/bin/env python3
"""
Training Data Generator for Container Resource Predictor

Generates synthetic workload patterns for training the resource prediction model.
Includes various workload types (web, batch, database) with noise and anomalies.
"""

import argparse
import numpy as np
from dataclasses import dataclass
from enum import Enum
from typing import Tuple
import os


class WorkloadType(Enum):
    WEB = "web"           # Diurnal pattern, bursty
    BATCH = "batch"       # Periodic spikes, predictable
    DATABASE = "database" # Steady with occasional spikes
    MICROSERVICE = "microservice"  # Variable, follows upstream
    CRON = "cron"         # Periodic, short-lived spikes


@dataclass
class WorkloadConfig:
    """Configuration for a workload type"""
    base_cpu: float       # Base CPU usage (0-1 normalized)
    base_memory: float    # Base memory usage (0-1 normalized)
    cpu_variance: float   # How much CPU varies
    memory_growth: float  # Memory growth rate (for leak simulation)
    burstiness: float     # How bursty the workload is
    diurnal_factor: float # How much time-of-day affects usage


WORKLOAD_CONFIGS = {
    WorkloadType.WEB: WorkloadConfig(
        base_cpu=0.3, base_memory=0.4, cpu_variance=0.3,
        memory_growth=0.0, burstiness=0.4, diurnal_factor=0.5
    ),
    WorkloadType.BATCH: WorkloadConfig(
        base_cpu=0.1, base_memory=0.2, cpu_variance=0.6,
        memory_growth=0.0, burstiness=0.7, diurnal_factor=0.1
    ),
    WorkloadType.DATABASE: WorkloadConfig(
        base_cpu=0.4, base_memory=0.6, cpu_variance=0.15,
        memory_growth=0.02, burstiness=0.2, diurnal_factor=0.2
    ),
    WorkloadType.MICROSERVICE: WorkloadConfig(
        base_cpu=0.2, base_memory=0.3, cpu_variance=0.25,
        memory_growth=0.0, burstiness=0.35, diurnal_factor=0.3
    ),
    WorkloadType.CRON: WorkloadConfig(
        base_cpu=0.05, base_memory=0.1, cpu_variance=0.8,
        memory_growth=0.0, burstiness=0.9, diurnal_factor=0.0
    ),
}


def generate_diurnal_pattern(hour: float, factor: float) -> float:
    """Generate diurnal (time-of-day) pattern multiplier"""
    # Peak at 14:00 (2 PM), trough at 04:00 (4 AM)
    peak_hour = 14.0 / 24.0
    return 1.0 + factor * np.sin(2 * np.pi * (hour - peak_hour + 0.25))


def generate_weekly_pattern(day: float, factor: float) -> float:
    """Generate weekly pattern multiplier (lower on weekends)"""
    # Weekdays (0-4) higher, weekends (5-6) lower
    if day > 5/7:  # Weekend
        return 1.0 - factor * 0.3
    return 1.0 + factor * 0.1


def generate_workload_sample(
    config: WorkloadConfig,
    hour: float,
    day: float,
    workload_age: float,
    add_anomaly: bool = False,
    anomaly_type: str = None
) -> Tuple[np.ndarray, np.ndarray]:
    """
    Generate a single training sample (features, labels).
    
    Features (12):
        0-2: cpu_usage_p50, p95, p99
        3-5: mem_usage_p50, p95, p99
        6: cpu_variance
        7: mem_trend
        8: throttle_ratio
        9: hour_of_day
        10: day_of_week
        11: workload_age_days
    
    Labels (5):
        0: cpu_request (normalized)
        1: cpu_limit (normalized)
        2: memory_request (normalized)
        3: memory_limit (normalized)
        4: confidence
    """
    # Apply temporal patterns
    diurnal = generate_diurnal_pattern(hour, config.diurnal_factor)
    weekly = generate_weekly_pattern(day, config.diurnal_factor * 0.5)
    temporal_mult = diurnal * weekly
    
    # Base usage with temporal adjustment
    base_cpu = config.base_cpu * temporal_mult
    base_mem = config.base_memory + config.memory_growth * workload_age
    
    # Add variance/noise
    cpu_noise = np.random.normal(0, config.cpu_variance * 0.3)
    mem_noise = np.random.normal(0, 0.05)
    
    # Generate percentiles (p50 < p95 < p99)
    cpu_p50 = np.clip(base_cpu + cpu_noise, 0.01, 0.95)
    cpu_p95 = np.clip(cpu_p50 * (1 + config.burstiness * 0.5 + np.random.uniform(0, 0.2)), cpu_p50, 0.98)
    cpu_p99 = np.clip(cpu_p95 * (1 + config.burstiness * 0.3 + np.random.uniform(0, 0.1)), cpu_p95, 1.0)
    
    mem_p50 = np.clip(base_mem + mem_noise, 0.01, 0.95)
    mem_p95 = np.clip(mem_p50 * (1 + 0.1 + np.random.uniform(0, 0.1)), mem_p50, 0.98)
    mem_p99 = np.clip(mem_p95 * (1 + 0.05 + np.random.uniform(0, 0.05)), mem_p95, 1.0)
    
    # CPU variance feature
    cpu_var = config.cpu_variance * (0.5 + np.random.uniform(0, 0.5))
    
    # Memory trend (positive = growing, negative = shrinking)
    mem_trend = config.memory_growth + np.random.normal(0, 0.02)
    
    # Throttle ratio (higher when CPU is constrained)
    throttle = np.clip(max(0, cpu_p99 - 0.8) * 2 + np.random.uniform(0, 0.1), 0, 1)

    # Handle anomalies
    confidence = 0.85 + np.random.uniform(0, 0.1)  # Base confidence
    
    if add_anomaly:
        if anomaly_type == "memory_leak":
            mem_trend = 0.3 + np.random.uniform(0, 0.2)  # Strong positive trend
            mem_p99 = np.clip(mem_p99 * 1.3, 0, 1.0)
            confidence *= 0.7  # Lower confidence for anomalous data
        elif anomaly_type == "cpu_spike":
            cpu_p99 = np.clip(cpu_p99 * 2.0, 0, 1.0)
            cpu_var *= 2.0
            throttle = np.clip(throttle * 2, 0, 1)
            confidence *= 0.75
        elif anomaly_type == "noisy":
            cpu_var *= 3.0
            confidence *= 0.6
    
    # Build feature vector
    features = np.array([
        cpu_p50, cpu_p95, cpu_p99,
        mem_p50, mem_p95, mem_p99,
        np.clip(cpu_var, 0, 1),
        np.clip(mem_trend, -1, 1),
        throttle,
        hour,
        day,
        np.clip(workload_age / 30.0, 0, 1)  # Normalize to 30 days
    ], dtype=np.float32)
    
    # Generate optimal resource recommendations (labels)
    # CPU request: slightly above p50 for headroom
    cpu_request = np.clip(cpu_p50 * 1.1 + 0.02, 0.01, 0.95)
    # CPU limit: above p99 with buffer
    cpu_limit = np.clip(cpu_p99 * 1.15 + 0.05, cpu_request, 1.0)
    
    # Memory request: above p50 with buffer
    mem_request = np.clip(mem_p50 * 1.15 + 0.02, 0.01, 0.95)
    # Memory limit: 20% buffer above p99 (per requirements)
    mem_limit = np.clip(mem_p99 * 1.20 + 0.05, mem_request, 1.0)
    
    labels = np.array([
        cpu_request,
        cpu_limit,
        mem_request,
        mem_limit,
        np.clip(confidence, 0, 1)
    ], dtype=np.float32)
    
    return features, labels


def generate_dataset(
    num_samples: int,
    anomaly_ratio: float = 0.1,
    seed: int = 42
) -> Tuple[np.ndarray, np.ndarray]:
    """
    Generate a complete training dataset.
    
    Args:
        num_samples: Number of samples to generate
        anomaly_ratio: Fraction of samples with anomalies
        seed: Random seed for reproducibility
    
    Returns:
        Tuple of (features, labels) arrays
    """
    np.random.seed(seed)
    
    features_list = []
    labels_list = []
    
    workload_types = list(WorkloadType)
    anomaly_types = ["memory_leak", "cpu_spike", "noisy"]
    
    for i in range(num_samples):
        # Random workload type
        wtype = np.random.choice(workload_types)
        config = WORKLOAD_CONFIGS[wtype]
        
        # Random temporal context
        hour = np.random.uniform(0, 1)
        day = np.random.uniform(0, 1)
        workload_age = np.random.uniform(0.1, 30)  # 0.1 to 30 days
        
        # Decide if this sample has an anomaly
        add_anomaly = np.random.random() < anomaly_ratio
        anomaly_type = np.random.choice(anomaly_types) if add_anomaly else None
        
        features, labels = generate_workload_sample(
            config, hour, day, workload_age, add_anomaly, anomaly_type
        )
        
        features_list.append(features)
        labels_list.append(labels)
        
        if (i + 1) % 10000 == 0:
            print(f"Generated {i + 1}/{num_samples} samples...")
    
    return np.array(features_list), np.array(labels_list)


def main():
    parser = argparse.ArgumentParser(description="Generate training data for resource predictor")
    parser.add_argument("--samples", type=int, default=100000, help="Number of samples")
    parser.add_argument("--anomaly-ratio", type=float, default=0.1, help="Fraction of anomalous samples")
    parser.add_argument("--seed", type=int, default=42, help="Random seed")
    parser.add_argument("--output", type=str, default="data/training_data.npz", help="Output file")
    args = parser.parse_args()
    
    print(f"Generating {args.samples} samples with {args.anomaly_ratio*100:.0f}% anomalies...")
    
    features, labels = generate_dataset(
        num_samples=args.samples,
        anomaly_ratio=args.anomaly_ratio,
        seed=args.seed
    )
    
    # Split into train/val/test (80/10/10)
    n = len(features)
    train_end = int(n * 0.8)
    val_end = int(n * 0.9)
    
    # Shuffle before splitting
    indices = np.random.permutation(n)
    features = features[indices]
    labels = labels[indices]
    
    train_features = features[:train_end]
    train_labels = labels[:train_end]
    val_features = features[train_end:val_end]
    val_labels = labels[train_end:val_end]
    test_features = features[val_end:]
    test_labels = labels[val_end:]
    
    # Create output directory
    os.makedirs(os.path.dirname(args.output) or ".", exist_ok=True)
    
    # Save dataset
    np.savez(
        args.output,
        train_features=train_features,
        train_labels=train_labels,
        val_features=val_features,
        val_labels=val_labels,
        test_features=test_features,
        test_labels=test_labels
    )
    
    print(f"\nDataset saved to {args.output}")
    print(f"  Training samples: {len(train_features)}")
    print(f"  Validation samples: {len(val_features)}")
    print(f"  Test samples: {len(test_features)}")
    print(f"\nFeature shape: {features.shape}")
    print(f"Label shape: {labels.shape}")
    
    # Print statistics
    print("\nFeature statistics (mean ± std):")
    feature_names = [
        "cpu_p50", "cpu_p95", "cpu_p99",
        "mem_p50", "mem_p95", "mem_p99",
        "cpu_var", "mem_trend", "throttle",
        "hour", "day", "age"
    ]
    for i, name in enumerate(feature_names):
        print(f"  {name}: {features[:, i].mean():.3f} ± {features[:, i].std():.3f}")
    
    print("\nLabel statistics (mean ± std):")
    label_names = ["cpu_req", "cpu_lim", "mem_req", "mem_lim", "confidence"]
    for i, name in enumerate(label_names):
        print(f"  {name}: {labels[:, i].mean():.3f} ± {labels[:, i].std():.3f}")


if __name__ == "__main__":
    main()
