#!/usr/bin/env python3
"""
Model Validation Script for Container Resource Predictor

Validates:
1. Inference latency (<5ms requirement)
2. Prediction accuracy on held-out test data
3. Model size (<100KB requirement)
"""

import argparse
import os
import time
import numpy as np
import onnxruntime as ort


def load_test_data(data_path: str):
    """Load test data from npz file"""
    data = np.load(data_path)
    return data["test_features"], data["test_labels"]


def validate_model_size(model_path: str, max_size_kb: float = 100.0) -> bool:
    """Validate model size is under limit"""
    size_bytes = os.path.getsize(model_path)
    size_kb = size_bytes / 1024
    
    print(f"\n=== Model Size Validation ===")
    print(f"Model path: {model_path}")
    print(f"Size: {size_bytes} bytes ({size_kb:.2f} KB)")
    print(f"Limit: {max_size_kb} KB")
    
    passed = size_kb <= max_size_kb
    print(f"Status: {'PASS ✓' if passed else 'FAIL ✗'}")
    
    return passed


def validate_inference_latency(
    session: ort.InferenceSession,
    test_features: np.ndarray,
    max_latency_ms: float = 5.0,
    num_iterations: int = 1000
) -> bool:
    """Validate inference latency is under limit"""
    print(f"\n=== Inference Latency Validation ===")
    print(f"Running {num_iterations} inference iterations...")
    
    # Warm up
    for _ in range(10):
        session.run(None, {"features": test_features[:1].astype(np.float32)})
    
    # Measure single-sample inference
    latencies = []
    for i in range(num_iterations):
        sample = test_features[i % len(test_features):i % len(test_features) + 1].astype(np.float32)
        start = time.perf_counter()
        session.run(None, {"features": sample})
        end = time.perf_counter()
        latencies.append((end - start) * 1000)  # Convert to ms
    
    latencies = np.array(latencies)
    mean_latency = np.mean(latencies)
    p50_latency = np.percentile(latencies, 50)
    p95_latency = np.percentile(latencies, 95)
    p99_latency = np.percentile(latencies, 99)
    max_latency = np.max(latencies)
    
    print(f"Mean latency: {mean_latency:.3f} ms")
    print(f"P50 latency:  {p50_latency:.3f} ms")
    print(f"P95 latency:  {p95_latency:.3f} ms")
    print(f"P99 latency:  {p99_latency:.3f} ms")
    print(f"Max latency:  {max_latency:.3f} ms")
    print(f"Limit: {max_latency_ms} ms")
    
    # Pass if P99 is under limit
    passed = p99_latency <= max_latency_ms
    print(f"Status: {'PASS ✓' if passed else 'FAIL ✗'}")
    
    return passed


def validate_prediction_accuracy(
    session: ort.InferenceSession,
    test_features: np.ndarray,
    test_labels: np.ndarray,
    max_mae: float = 0.1
) -> bool:
    """Validate prediction accuracy on test data"""
    print(f"\n=== Prediction Accuracy Validation ===")
    
    # Run inference on all test data
    predictions = session.run(None, {"features": test_features.astype(np.float32)})[0]
    
    # Calculate metrics
    mae = np.mean(np.abs(predictions - test_labels), axis=0)
    mse = np.mean((predictions - test_labels) ** 2, axis=0)
    rmse = np.sqrt(mse)
    
    label_names = ["cpu_req", "cpu_lim", "mem_req", "mem_lim", "confidence"]
    
    print(f"Test samples: {len(test_features)}")
    print(f"\nPer-output metrics:")
    print(f"{'Output':<12} {'MAE':>8} {'RMSE':>8}")
    print("-" * 30)
    
    for i, name in enumerate(label_names):
        print(f"{name:<12} {mae[i]:>8.4f} {rmse[i]:>8.4f}")
    
    overall_mae = np.mean(mae)
    overall_rmse = np.mean(rmse)
    
    print("-" * 30)
    print(f"{'Overall':<12} {overall_mae:>8.4f} {overall_rmse:>8.4f}")
    print(f"\nMAE limit: {max_mae}")
    
    passed = overall_mae <= max_mae
    print(f"Status: {'PASS ✓' if passed else 'FAIL ✗'}")
    
    # Additional analysis
    print(f"\n=== Prediction Distribution Analysis ===")
    print(f"{'Output':<12} {'Pred Mean':>10} {'True Mean':>10} {'Pred Std':>10} {'True Std':>10}")
    print("-" * 55)
    
    for i, name in enumerate(label_names):
        pred_mean = np.mean(predictions[:, i])
        true_mean = np.mean(test_labels[:, i])
        pred_std = np.std(predictions[:, i])
        true_std = np.std(test_labels[:, i])
        print(f"{name:<12} {pred_mean:>10.4f} {true_mean:>10.4f} {pred_std:>10.4f} {true_std:>10.4f}")
    
    return passed


def main():
    parser = argparse.ArgumentParser(description="Validate resource predictor model")
    parser.add_argument("--model", type=str, default="models/predictor.onnx", help="ONNX model path")
    parser.add_argument("--data", type=str, default="data/training_data.npz", help="Test data path")
    parser.add_argument("--max-latency-ms", type=float, default=5.0, help="Max inference latency (ms)")
    parser.add_argument("--max-size-kb", type=float, default=100.0, help="Max model size (KB)")
    parser.add_argument("--max-mae", type=float, default=0.1, help="Max mean absolute error")
    args = parser.parse_args()
    
    print("=" * 60)
    print("Container Resource Predictor - Model Validation")
    print("=" * 60)
    
    # Load model
    print(f"\nLoading model from {args.model}...")
    session = ort.InferenceSession(args.model, providers=["CPUExecutionProvider"])
    
    # Load test data
    print(f"Loading test data from {args.data}...")
    test_features, test_labels = load_test_data(args.data)
    print(f"Test samples: {len(test_features)}")
    
    # Run validations
    results = {}
    results["size"] = validate_model_size(args.model, args.max_size_kb)
    results["latency"] = validate_inference_latency(session, test_features, args.max_latency_ms)
    results["accuracy"] = validate_prediction_accuracy(session, test_features, test_labels, args.max_mae)
    
    # Summary
    print("\n" + "=" * 60)
    print("VALIDATION SUMMARY")
    print("=" * 60)
    
    all_passed = all(results.values())
    for name, passed in results.items():
        status = "PASS ✓" if passed else "FAIL ✗"
        print(f"  {name.capitalize()}: {status}")
    
    print("-" * 60)
    print(f"Overall: {'ALL TESTS PASSED ✓' if all_passed else 'SOME TESTS FAILED ✗'}")
    print("=" * 60)
    
    return 0 if all_passed else 1


if __name__ == "__main__":
    exit(main())
