# Container Resource Predictor ML Model

This directory contains the training pipeline for the resource prediction model.

## Model Architecture

- **Input**: 12 features (normalized 0-1)
  - CPU percentiles (p50, p95, p99)
  - Memory percentiles (p50, p95, p99)
  - CPU variance, memory trend, throttle ratio
  - Temporal features (hour, day, workload age)

- **Architecture**: 
  - Dense(32, ReLU) → Dense(16, ReLU) → Output(5)
  - ~2,500 parameters (<100KB)
  - Quantized to int8 for edge inference

- **Output**: 5 values
  - CPU request (normalized)
  - CPU limit (normalized)
  - Memory request (normalized)
  - Memory limit (normalized)
  - Confidence score (0-1)

## Usage

```bash
# Install dependencies
pip install -r requirements.txt

# Generate training data
python generate_data.py --samples 100000 --output data/training_data.npz

# Train model
python train.py --data data/training_data.npz --output models/

# Validate model
python validate.py --model models/predictor.onnx

# Export for deployment
python export.py --model models/predictor.onnx --output ../resource-agent/models/
```

## Model Versioning

Models are versioned using semantic versioning: `v{major}.{minor}.{patch}`

### Version Semantics
- **Major**: Breaking changes to input/output format (feature count, output shape)
- **Minor**: Architecture changes, significant retraining, new workload types
- **Patch**: Bug fixes, minor accuracy improvements, hyperparameter tuning

### Version History
| Version | Date | Changes |
|---------|------|---------|
| v1.0.0 | 2024-12-21 | Initial release with 5 workload types |

### Model Files
- `predictor_v{version}.onnx` - Versioned model file
- `predictor.onnx` - Symlink/copy to current active version
- `manifest.json` - Model metadata and checksums

### Deployment
The model can be deployed in two ways:
1. **Embedded**: Model bytes compiled into the Rust binary (see `embedded_model.rs`)
2. **External**: Model loaded from filesystem at runtime

### Rollback
To rollback to a previous version:
1. Update `predictor.onnx` to point to the desired version
2. Update `manifest.json` with the correct version info
3. Restart the resource-agent pods

The recommendation-api stores the last 5 model versions for rollback capability.
