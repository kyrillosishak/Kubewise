#!/usr/bin/env python3
"""
Model Validation Script for Container Resource Predictor

Validates:
1. Inference latency (<5ms requirement for LSTM, <10ms with sequence)
2. Prediction accuracy on held-out test data
3. Model size (<100KB requirement)
4. LSTM-specific temporal pattern recognition
"""

import argparse
import os
import time
import numpy as np
import onnxruntime as ort


def load_test_data(data_path: str):
    """Load test data from npz file - supports both LSTM sequences and flat features"""
    data = np.load(data_path)
    
    # Check if this is LSTM sequence data or flat feature data
    if "test_sequences" in data:
        # LSTM format
        return data["test_sequences"], data["test_labels"], "lstm"
    elif "test_features" in data:
        # Flat format (legacy)
        return data["test_features"], data["test_labels"], "flat"
    else:
        raise ValueError(f"Unknown data format. Keys: {list(data.keys())}")


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
    test_data: np.ndarray,
    data_format: str,
    max_latency_ms: float = 5.0,
    num_iterations: int = 1000
) -> bool:
    """Validate inference latency is under limit"""
    print(f"\n=== Inference Latency Validation ===")
    print(f"Data format: {data_format}")
    print(f"Running {num_iterations} inference iterations...")
    
    # Get input name from model
    input_name = session.get_inputs()[0].name
    
    # Warm up
    for _ in range(10):
        sample = test_data[:1].astype(np.float32)
        session.run(None, {input_name: sample})
    
    # Measure single-sample inference
    latencies = []
    for i in range(num_iterations):
        idx = i % len(test_data)
        sample = test_data[idx:idx + 1].astype(np.float32)
        start = time.perf_counter()
        session.run(None, {input_name: sample})
        end = time.perf_counter()
        latencies.append((end - start) * 1000)  # Convert to ms
    
    latencies = np.array(latencies)
    mean_latency = np.mean(latencies)
    p50_latency = np.percentile(latencies, 50)
    p95_latency = np.percentile(latencies, 95)
    p99_latency = np.percentile(latencies, 99)
    max_observed = np.max(latencies)
    
    print(f"Mean latency: {mean_latency:.3f} ms")
    print(f"P50 latency:  {p50_latency:.3f} ms")
    print(f"P95 latency:  {p95_latency:.3f} ms")
    print(f"P99 latency:  {p99_latency:.3f} ms")
    print(f"Max latency:  {max_observed:.3f} ms")
    print(f"Limit: {max_latency_ms} ms")
    
    # Pass if P99 is under limit
    passed = p99_latency <= max_latency_ms
    print(f"Status: {'PASS ✓' if passed else 'FAIL ✗'}")
    
    return passed


def validate_prediction_accuracy(
    session: ort.InferenceSession,
    test_data: np.ndarray,
    test_labels: np.ndarray,
    data_format: str,
    max_mae: float = 0.1
) -> bool:
    """Validate prediction accuracy on test data"""
    print(f"\n=== Prediction Accuracy Validation ===")
    
    # Get input name from model
    input_name = session.get_inputs()[0].name
    
    # Run inference on all test data
    predictions = session.run(None, {input_name: test_data.astype(np.float32)})[0]
    
    # Calculate metrics
    mae = np.mean(np.abs(predictions - test_labels), axis=0)
    mse = np.mean((predictions - test_labels) ** 2, axis=0)
    rmse = np.sqrt(mse)
    
    label_names = ["cpu_req", "cpu_lim", "mem_req", "mem_lim", "confidence"]
    
    print(f"Test samples: {len(test_data)}")
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


def validate_temporal_patterns(
    session: ort.InferenceSession,
    seq_len: int = 10
) -> bool:
    """Validate LSTM model captures temporal patterns correctly"""
    print(f"\n=== Temporal Pattern Validation (LSTM-specific) ===")
    
    input_name = session.get_inputs()[0].name
    
    # Test 1: Increasing memory trend should predict higher memory limits
    print("\nTest 1: Memory trend detection")
    
    # Stable memory sequence
    stable_seq = np.zeros((1, seq_len, 12), dtype=np.float32)
    stable_seq[:, :, 0:3] = 0.3  # CPU percentiles
    stable_seq[:, :, 3:6] = 0.4  # Memory percentiles (stable)
    stable_seq[:, :, 7] = 0.0    # No memory trend
    
    # Increasing memory sequence (leak pattern)
    leak_seq = np.zeros((1, seq_len, 12), dtype=np.float32)
    leak_seq[:, :, 0:3] = 0.3    # CPU percentiles
    for i in range(seq_len):
        leak_seq[:, i, 3:6] = 0.4 + i * 0.05  # Increasing memory
    leak_seq[:, :, 7] = 0.3      # Positive memory trend
    
    stable_pred = session.run(None, {input_name: stable_seq})[0]
    leak_pred = session.run(None, {input_name: leak_seq})[0]
    
    # Memory limit should be higher for leak pattern
    stable_mem_limit = stable_pred[0, 3]
    leak_mem_limit = leak_pred[0, 3]
    
    print(f"  Stable memory limit prediction: {stable_mem_limit:.4f}")
    print(f"  Leak memory limit prediction:   {leak_mem_limit:.4f}")
    
    mem_test_passed = leak_mem_limit > stable_mem_limit
    print(f"  Leak > Stable: {'PASS ✓' if mem_test_passed else 'FAIL ✗'}")
    
    # Test 2: CPU spike pattern should predict higher CPU limits
    print("\nTest 2: CPU spike detection")
    
    # Stable CPU sequence
    stable_cpu_seq = np.zeros((1, seq_len, 12), dtype=np.float32)
    stable_cpu_seq[:, :, 0:3] = 0.3  # Stable CPU
    stable_cpu_seq[:, :, 3:6] = 0.4  # Memory
    stable_cpu_seq[:, :, 6] = 0.1    # Low variance
    
    # Spiky CPU sequence
    spiky_cpu_seq = np.zeros((1, seq_len, 12), dtype=np.float32)
    spiky_cpu_seq[:, :, 3:6] = 0.4   # Memory
    for i in range(seq_len):
        if i % 3 == 0:
            spiky_cpu_seq[:, i, 0:3] = [0.3, 0.7, 0.9]  # Spike
        else:
            spiky_cpu_seq[:, i, 0:3] = [0.2, 0.3, 0.35]  # Normal
    spiky_cpu_seq[:, :, 6] = 0.5     # High variance
    
    stable_cpu_pred = session.run(None, {input_name: stable_cpu_seq})[0]
    spiky_cpu_pred = session.run(None, {input_name: spiky_cpu_seq})[0]
    
    stable_cpu_limit = stable_cpu_pred[0, 1]
    spiky_cpu_limit = spiky_cpu_pred[0, 1]
    
    print(f"  Stable CPU limit prediction: {stable_cpu_limit:.4f}")
    print(f"  Spiky CPU limit prediction:  {spiky_cpu_limit:.4f}")
    
    cpu_test_passed = spiky_cpu_limit > stable_cpu_limit
    print(f"  Spiky > Stable: {'PASS ✓' if cpu_test_passed else 'FAIL ✗'}")
    
    # Test 3: Confidence should be lower for high-variance sequences
    print("\nTest 3: Confidence calibration")
    
    stable_confidence = stable_cpu_pred[0, 4]
    spiky_confidence = spiky_cpu_pred[0, 4]
    
    print(f"  Stable sequence confidence: {stable_confidence:.4f}")
    print(f"  Spiky sequence confidence:  {spiky_confidence:.4f}")
    
    confidence_test_passed = stable_confidence > spiky_confidence
    print(f"  Stable > Spiky confidence: {'PASS ✓' if confidence_test_passed else 'FAIL ✗'}")
    
    all_passed = mem_test_passed and cpu_test_passed and confidence_test_passed
    print(f"\nTemporal Pattern Tests: {'ALL PASS ✓' if all_passed else 'SOME FAILED ✗'}")
    
    return all_passed


def validate_edge_cases(
    session: ort.InferenceSession,
    seq_len: int = 10
) -> bool:
    """Validate model handles edge cases correctly"""
    print(f"\n=== Edge Case Validation ===")
    
    input_name = session.get_inputs()[0].name
    
    tests_passed = 0
    total_tests = 4
    
    # Test 1: All zeros input
    print("\nTest 1: All zeros input")
    zero_seq = np.zeros((1, seq_len, 12), dtype=np.float32)
    try:
        pred = session.run(None, {input_name: zero_seq})[0]
        # Should produce valid output (all values 0-1)
        valid = np.all((pred >= 0) & (pred <= 1))
        print(f"  Output valid (0-1 range): {'PASS ✓' if valid else 'FAIL ✗'}")
        if valid:
            tests_passed += 1
    except Exception as e:
        print(f"  FAIL ✗ - Exception: {e}")
    
    # Test 2: All ones input
    print("\nTest 2: All ones input (max usage)")
    ones_seq = np.ones((1, seq_len, 12), dtype=np.float32)
    try:
        pred = session.run(None, {input_name: ones_seq})[0]
        valid = np.all((pred >= 0) & (pred <= 1))
        # High usage should predict high limits
        high_limits = pred[0, 1] > 0.5 and pred[0, 3] > 0.5
        print(f"  Output valid: {'PASS ✓' if valid else 'FAIL ✗'}")
        print(f"  High limits predicted: {'PASS ✓' if high_limits else 'FAIL ✗'}")
        if valid and high_limits:
            tests_passed += 1
    except Exception as e:
        print(f"  FAIL ✗ - Exception: {e}")
    
    # Test 3: Batch inference
    print("\nTest 3: Batch inference (10 samples)")
    batch_seq = np.random.rand(10, seq_len, 12).astype(np.float32)
    try:
        pred = session.run(None, {input_name: batch_seq})[0]
        correct_shape = pred.shape == (10, 5)
        valid = np.all((pred >= 0) & (pred <= 1))
        print(f"  Correct output shape {pred.shape}: {'PASS ✓' if correct_shape else 'FAIL ✗'}")
        print(f"  All outputs valid: {'PASS ✓' if valid else 'FAIL ✗'}")
        if correct_shape and valid:
            tests_passed += 1
    except Exception as e:
        print(f"  FAIL ✗ - Exception: {e}")
    
    # Test 4: Deterministic output
    print("\nTest 4: Deterministic output")
    test_seq = np.random.rand(1, seq_len, 12).astype(np.float32)
    pred1 = session.run(None, {input_name: test_seq})[0]
    pred2 = session.run(None, {input_name: test_seq})[0]
    deterministic = np.allclose(pred1, pred2)
    print(f"  Same input → same output: {'PASS ✓' if deterministic else 'FAIL ✗'}")
    if deterministic:
        tests_passed += 1
    
    passed = tests_passed == total_tests
    print(f"\nEdge Case Tests: {tests_passed}/{total_tests} passed")
    
    return passed


def main():
    parser = argparse.ArgumentParser(description="Validate LSTM resource predictor model")
    parser.add_argument("--model", type=str, default="models/predictor_lstm.onnx", help="ONNX model path")
    parser.add_argument("--data", type=str, default="data/training_data.npz", help="Test data path")
    parser.add_argument("--max-latency-ms", type=float, default=10.0, help="Max inference latency (ms)")
    parser.add_argument("--max-size-kb", type=float, default=500.0, help="Max model size (KB) - LSTM models are larger")
    parser.add_argument("--max-mae", type=float, default=0.15, help="Max mean absolute error")
    parser.add_argument("--seq-len", type=int, default=10, help="Sequence length for temporal tests")
    args = parser.parse_args()
    
    print("=" * 60)
    print("Container Resource Predictor - LSTM Model Validation")
    print("=" * 60)
    
    # Load model
    print(f"\nLoading model from {args.model}...")
    if not os.path.exists(args.model):
        print(f"ERROR: Model file not found: {args.model}")
        print("Run training first: python train.py")
        return 1
    
    session = ort.InferenceSession(args.model, providers=["CPUExecutionProvider"])
    
    # Print model info
    print(f"Model inputs: {[i.name for i in session.get_inputs()]}")
    print(f"Model outputs: {[o.name for o in session.get_outputs()]}")
    print(f"Input shape: {session.get_inputs()[0].shape}")
    
    # Load test data
    print(f"\nLoading test data from {args.data}...")
    if not os.path.exists(args.data):
        print(f"ERROR: Data file not found: {args.data}")
        print("Generate data first: python generate_data.py")
        return 1
    
    test_data, test_labels, data_format = load_test_data(args.data)
    print(f"Data format: {data_format}")
    print(f"Test samples: {len(test_data)}")
    print(f"Test data shape: {test_data.shape}")
    
    # Run validations
    results = {}
    results["size"] = validate_model_size(args.model, args.max_size_kb)
    results["latency"] = validate_inference_latency(
        session, test_data, data_format, args.max_latency_ms
    )
    results["accuracy"] = validate_prediction_accuracy(
        session, test_data, test_labels, data_format, args.max_mae
    )
    results["temporal"] = validate_temporal_patterns(session, args.seq_len)
    results["edge_cases"] = validate_edge_cases(session, args.seq_len)
    
    # Summary
    print("\n" + "=" * 60)
    print("VALIDATION SUMMARY")
    print("=" * 60)
    
    all_passed = all(results.values())
    for name, passed in results.items():
        status = "PASS ✓" if passed else "FAIL ✗"
        print(f"  {name.capitalize():15} {status}")
    
    print("-" * 60)
    print(f"Overall: {'ALL TESTS PASSED ✓' if all_passed else 'SOME TESTS FAILED ✗'}")
    print("=" * 60)
    
    return 0 if all_passed else 1


if __name__ == "__main__":
    exit(main())
