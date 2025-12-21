#!/usr/bin/env python3
"""
Training Data Generator for Container Resource Predictor

Generates synthetic workload patterns with temporal sequences for training 
the LSTM-based resource prediction model. Includes various workload types 
(web, batch, database) with realistic temporal patterns, noise, and anomalies.
"""

import argparse
import numpy as np
from dataclasses import dataclass
from enum import Enum
from typing import Tuple, List
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
    autocorrelation: float  # How correlated consecutive samples are


WORKLOAD_CONFIGS = {
    WorkloadType.WEB: WorkloadConfig(
        base_cpu=0.3, base_memory=0.4, cpu_variance=0.3,
        memory_growth=0.0, burstiness=0.4, diurnal_factor=0.5,
        autocorrelation=0.7
    ),
    WorkloadType.BATCH: WorkloadConfig(
        base_cpu=0.1, base_memory=0.2, cpu_variance=0.6,
        memory_growth=0.0, burstiness=0.7, diurnal_factor=0.1,
        autocorrelation=0.9
    ),
    WorkloadType.DATABASE: WorkloadConfig(
        base_cpu=0.4, base_memory=0.6, cpu_variance=0.15,
        memory_growth=0.02, burstiness=0.2, diurnal_factor=0.2,
        autocorrelation=0.85
    ),
    WorkloadType.MICROSERVICE: WorkloadConfig(
        base_cpu=0.2, base_memory=0.3, cpu_variance=0.25,
        memory_growth=0.0, burstiness=0.35, diurnal_factor=0.3,
        autocorrelation=0.6
    ),
    WorkloadType.CRON: WorkloadConfig(
        base_cpu=0.05, base_memory=0.1, cpu_variance=0.8,
        memory_growth=0.0, burstiness=0.9, diurnal_factor=0.0,
        autocorrelation=0.3
    ),
}


def generate_diurnal_pattern(hour: float, factor: float) -> float:
    """Generate diurnal (time-of-day) pattern multiplier"""
    peak_hour = 14.0 / 24.0
    return 1.0 + factor * np.sin(2 * np.pi * (hour - peak_hour + 0.25))


def generate_weekly_pattern(day: float, factor: float) -> float:
    """Generate weekly pattern multiplier (lower on weekends)"""
    if day > 5/7:
        return 1.0 - factor * 0.3
    return 1.0 + factor * 0.1


def generate_single_timestep(
    config: WorkloadConfig,
    hour: float,
    day: float,
    workload_age: float,
    prev_cpu: float = None,
    prev_mem: float = None,
    add_anomaly: bool = False,
    anomaly_type: str = None
) -> np.ndarray:
    """
    Generate a single timestep of features.
    
    Features (12):
        0-2: cpu_usage_p50, p95, p99
        3-5: mem_usage_p50, p95, p99
        6: cpu_variance
        7: mem_trend
        8: throttle_ratio
        9: hour_of_day
        10: day_of_week
        11: workload_age_days
    """
    # Apply temporal patterns
    diurnal = generate_diurnal_pattern(hour, config.diurnal_factor)
    weekly = generate_weekly_pattern(day, config.diurnal_factor * 0.5)
    temporal_mult = diurnal * weekly
    
    # Base usage with temporal adjustment
    base_cpu = config.base_cpu * temporal_mult
    base_mem = config.base_memory + config.memory_growth * workload_age
    
    # Apply autocorrelation if we have previous values
    if prev_cpu is not None:
        base_cpu = config.autocorrelation * prev_cpu + (1 - config.autocorrelation) * base_cpu
    if prev_mem is not None:
        base_mem = config.autocorrelation * prev_mem + (1 - config.autocorrelation) * base_mem
    
    # Add variance/noise
    cpu_noise = np.random.normal(0, config.cpu_variance * 0.3)
    mem_noise = np.random.normal(0, 0.05)
    
    # Generate percentiles
    cpu_p50 = np.clip(base_cpu + cpu_noise, 0.01, 0.95)
    cpu_p95 = np.clip(cpu_p50 * (1 + config.burstiness * 0.5 + np.random.uniform(0, 0.2)), cpu_p50, 0.98)
    cpu_p99 = np.clip(cpu_p95 * (1 + config.burstiness * 0.3 + np.random.uniform(0, 0.1)), cpu_p95, 1.0)
    
    mem_p50 = np.clip(base_mem + mem_noise, 0.01, 0.95)
    mem_p95 = np.clip(mem_p50 * (1 + 0.1 + np.random.uniform(0, 0.1)), mem_p50, 0.98)
    mem_p99 = np.clip(mem_p95 * (1 + 0.05 + np.random.uniform(0, 0.05)), mem_p95, 1.0)
    
    # CPU variance feature
    cpu_var = config.cpu_variance * (0.5 + np.random.uniform(0, 0.5))
    
    # Memory trend
    mem_trend = config.memory_growth + np.random.normal(0, 0.02)
    
    # Throttle ratio
    throttle = np.clip(max(0, cpu_p99 - 0.8) * 2 + np.random.uniform(0, 0.1), 0, 1)
    
    # Handle anomalies
    if add_anomaly:
        if anomaly_type == "memory_leak":
            mem_trend = 0.3 + np.random.uniform(0, 0.2)
            mem_p99 = np.clip(mem_p99 * 1.3, 0, 1.0)
        elif anomaly_type == "cpu_spike":
            cpu_p99 = np.clip(cpu_p99 * 2.0, 0, 1.0)
            cpu_var *= 2.0
            throttle = np.clip(throttle * 2, 0, 1)
        elif anomaly_type == "noisy":
            cpu_var *= 3.0
    
    features = np.array([
        cpu_p50, cpu_p95, cpu_p99,
        mem_p50, mem_p95, mem_p99,
        np.clip(cpu_var, 0, 1),
        np.clip(mem_trend, -1, 1),
        throttle,
        hour,
        day,
        np.clip(workload_age / 30.0, 0, 1)
    ], dtype=np.float32)
    
    return features, cpu_p50, mem_p50


def generate_sequence(
    config: WorkloadConfig,
    seq_len: int,
    start_hour: float,
    start_day: float,
    workload_age: float,
    time_step_hours: float = 0.5,  # 30 minutes between samples
    add_anomaly: bool = False,
    anomaly_type: str = None
) -> Tuple[np.ndarray, np.ndarray]:
    """
    Generate a sequence of timesteps for LSTM training.
    
    Returns:
        sequence: (seq_len, 12) array of features
        labels: (5,) array of optimal resource recommendations
    """
    sequence = []
    prev_cpu, prev_mem = None, None
    
    # Track values for label generation
    all_cpu_p99 = []
    all_mem_p99 = []
    all_cpu_p50 = []
    all_mem_p50 = []
    
    for i in range(seq_len):
        # Advance time
        hour = (start_hour + i * time_step_hours / 24.0) % 1.0
        day = (start_day + i * time_step_hours / (24.0 * 7)) % 1.0
        age = workload_age + i * time_step_hours / 24.0
        
        # Only add anomaly in later part of sequence
        step_anomaly = add_anomaly and i >= seq_len // 2
        
        features, prev_cpu, prev_mem = generate_single_timestep(
            config, hour, day, age, prev_cpu, prev_mem,
            step_anomaly, anomaly_type
        )
        sequence.append(features)
        
        all_cpu_p50.append(features[0])
        all_cpu_p99.append(features[2])
        all_mem_p50.append(features[3])
        all_mem_p99.append(features[5])
    
    sequence = np.array(sequence, dtype=np.float32)
    
    # Generate labels based on sequence statistics
    cpu_p50_avg = np.mean(all_cpu_p50)
    cpu_p99_max = np.max(all_cpu_p99)
    mem_p50_avg = np.mean(all_mem_p50)
    mem_p99_max = np.max(all_mem_p99)
    
    # CPU request: above average p50
    cpu_request = np.clip(cpu_p50_avg * 1.1 + 0.02, 0.01, 0.95)
    # CPU limit: above max p99
    cpu_limit = np.clip(cpu_p99_max * 1.15 + 0.05, cpu_request, 1.0)
    # Memory request: above average p50
    mem_request = np.clip(mem_p50_avg * 1.15 + 0.02, 0.01, 0.95)
    # Memory limit: 20% buffer above max p99
    mem_limit = np.clip(mem_p99_max * 1.20 + 0.05, mem_request, 1.0)
    
    # Confidence based on variance in sequence
    cpu_stability = 1.0 - np.std(all_cpu_p99) / (np.mean(all_cpu_p99) + 0.01)
    mem_stability = 1.0 - np.std(all_mem_p99) / (np.mean(all_mem_p99) + 0.01)
    confidence = np.clip(0.5 * (cpu_stability + mem_stability) * 0.9 + 0.1, 0.3, 0.95)
    
    if add_anomaly:
        confidence *= 0.7
    
    labels = np.array([
        cpu_request, cpu_limit, mem_request, mem_limit, confidence
    ], dtype=np.float32)
    
    return sequence, labels


def generate_dataset(
    num_samples: int,
    seq_len: int = 10,
    anomaly_ratio: float = 0.1,
    seed: int = 42
) -> Tuple[np.ndarray, np.ndarray]:
    """
    Generate a complete training dataset with sequences.
    
    Args:
        num_samples: Number of sequences to generate
        seq_len: Length of each sequence
        anomaly_ratio: Fraction of samples with anomalies
        seed: Random seed for reproducibility
    
    Returns:
        Tuple of (sequences, labels) arrays
        sequences shape: (num_samples, seq_len, 12)
        labels shape: (num_samples, 5)
    """
    np.random.seed(seed)
    
    sequences_list = []
    labels_list = []
    
    workload_types = list(WorkloadType)
    anomaly_types = ["memory_leak", "cpu_spike", "noisy"]
    
    for i in range(num_samples):
        # Random workload type
        wtype = np.random.choice(workload_types)
        config = WORKLOAD_CONFIGS[wtype]
        
        # Random starting temporal context
        start_hour = np.random.uniform(0, 1)
        start_day = np.random.uniform(0, 1)
        workload_age = np.random.uniform(0.1, 30)
        
        # Decide if this sample has an anomaly
        add_anomaly = np.random.random() < anomaly_ratio
        anomaly_type = np.random.choice(anomaly_types) if add_anomaly else None
        
        sequence, labels = generate_sequence(
            config, seq_len, start_hour, start_day, workload_age,
            add_anomaly=add_anomaly, anomaly_type=anomaly_type
        )
        
        sequences_list.append(sequence)
        labels_list.append(labels)
        
        if (i + 1) % 10000 == 0:
            print(f"Generated {i + 1}/{num_samples} sequences...")
    
    return np.array(sequences_list), np.array(labels_list)


def main():
    parser = argparse.ArgumentParser(description="Generate training data for LSTM resource predictor")
    parser.add_argument("--samples", type=int, default=100000, help="Number of sequences")
    parser.add_argument("--seq-len", type=int, default=10, help="Sequence length")
    parser.add_argument("--anomaly-ratio", type=float, default=0.1, help="Fraction of anomalous samples")
    parser.add_argument("--seed", type=int, default=42, help="Random seed")
    parser.add_argument("--output", type=str, default="data/training_data.npz", help="Output file")
    args = parser.parse_args()
    
    print(f"Generating {args.samples} sequences (length={args.seq_len}) with {args.anomaly_ratio*100:.0f}% anomalies...")
    print(f"This will create data for LSTM training.\n")
    
    sequences, labels = generate_dataset(
        num_samples=args.samples,
        seq_len=args.seq_len,
        anomaly_ratio=args.anomaly_ratio,
        seed=args.seed
    )
    
    # Split into train/val/test (80/10/10)
    n = len(sequences)
    train_end = int(n * 0.8)
    val_end = int(n * 0.9)
    
    # Shuffle before splitting
    indices = np.random.permutation(n)
    sequences = sequences[indices]
    labels = labels[indices]
    
    train_sequences = sequences[:train_end]
    train_labels = labels[:train_end]
    val_sequences = sequences[train_end:val_end]
    val_labels = labels[train_end:val_end]
    test_sequences = sequences[val_end:]
    test_labels = labels[val_end:]
    
    # Create output directory
    os.makedirs(os.path.dirname(args.output) or ".", exist_ok=True)
    
    # Save dataset
    np.savez(
        args.output,
        train_sequences=train_sequences,
        train_labels=train_labels,
        val_sequences=val_sequences,
        val_labels=val_labels,
        test_sequences=test_sequences,
        test_labels=test_labels,
        seq_len=args.seq_len
    )
    
    print(f"\nDataset saved to {args.output}")
    print(f"  Training sequences: {len(train_sequences)}")
    print(f"  Validation sequences: {len(val_sequences)}")
    print(f"  Test sequences: {len(test_sequences)}")
    print(f"\nSequence shape: {sequences.shape} (samples, seq_len, features)")
    print(f"Label shape: {labels.shape}")
    
    # Print statistics
    print("\nFeature statistics (mean ± std across all timesteps):")
    feature_names = [
        "cpu_p50", "cpu_p95", "cpu_p99",
        "mem_p50", "mem_p95", "mem_p99",
        "cpu_var", "mem_trend", "throttle",
        "hour", "day", "age"
    ]
    flat_features = sequences.reshape(-1, 12)
    for i, name in enumerate(feature_names):
        print(f"  {name}: {flat_features[:, i].mean():.3f} ± {flat_features[:, i].std():.3f}")
    
    print("\nLabel statistics (mean ± std):")
    label_names = ["cpu_req", "cpu_lim", "mem_req", "mem_lim", "confidence"]
    for i, name in enumerate(label_names):
        print(f"  {name}: {labels[:, i].mean():.3f} ± {labels[:, i].std():.3f}")


if __name__ == "__main__":
    main()
