#!/usr/bin/env python3
"""
Training Script for Container Resource Predictor Model

Trains a lightweight neural network for resource prediction and exports to ONNX.
Model is quantized to int8 for efficient edge inference.
"""

import argparse
import os
import numpy as np
import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import DataLoader, TensorDataset


class ResourcePredictor(nn.Module):
    """
    Lightweight neural network for resource prediction.
    
    Architecture:
        Input(12) -> Dense(32, ReLU) -> Dense(16, ReLU) -> Output(5)
    
    ~2,500 parameters, designed for <5ms inference on edge devices.
    """
    
    def __init__(self, input_size: int = 12, output_size: int = 5):
        super().__init__()
        self.network = nn.Sequential(
            nn.Linear(input_size, 32),
            nn.ReLU(),
            nn.Linear(32, 16),
            nn.ReLU(),
            nn.Linear(16, output_size),
            nn.Sigmoid()  # Output normalized 0-1
        )
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.network(x)
    
    def count_parameters(self) -> int:
        return sum(p.numel() for p in self.parameters() if p.requires_grad)


def load_data(data_path: str, batch_size: int = 256):
    """Load training data from npz file"""
    data = np.load(data_path)
    
    train_features = torch.FloatTensor(data["train_features"])
    train_labels = torch.FloatTensor(data["train_labels"])
    val_features = torch.FloatTensor(data["val_features"])
    val_labels = torch.FloatTensor(data["val_labels"])
    
    train_dataset = TensorDataset(train_features, train_labels)
    val_dataset = TensorDataset(val_features, val_labels)
    
    train_loader = DataLoader(train_dataset, batch_size=batch_size, shuffle=True)
    val_loader = DataLoader(val_dataset, batch_size=batch_size, shuffle=False)
    
    return train_loader, val_loader


def train_epoch(model, train_loader, criterion, optimizer, device):
    """Train for one epoch"""
    model.train()
    total_loss = 0.0
    
    for features, labels in train_loader:
        features, labels = features.to(device), labels.to(device)
        
        optimizer.zero_grad()
        outputs = model(features)
        loss = criterion(outputs, labels)
        loss.backward()
        optimizer.step()
        
        total_loss += loss.item() * features.size(0)
    
    return total_loss / len(train_loader.dataset)


def validate(model, val_loader, criterion, device):
    """Validate model"""
    model.eval()
    total_loss = 0.0
    
    with torch.no_grad():
        for features, labels in val_loader:
            features, labels = features.to(device), labels.to(device)
            outputs = model(features)
            loss = criterion(outputs, labels)
            total_loss += loss.item() * features.size(0)
    
    return total_loss / len(val_loader.dataset)


def export_onnx(model, output_path: str, input_size: int = 12):
    """Export model to ONNX format"""
    import onnx
    
    model.eval()
    dummy_input = torch.randn(1, input_size)
    
    # Export using dynamo=False for simpler export
    torch.onnx.export(
        model,
        dummy_input,
        output_path,
        export_params=True,
        opset_version=17,
        do_constant_folding=True,
        input_names=["features"],
        output_names=["predictions"],
        dynamic_axes={
            "features": {0: "batch_size"},
            "predictions": {0: "batch_size"}
        },
        dynamo=False
    )
    
    # Run shape inference to fix any shape issues
    onnx_model = onnx.load(output_path)
    onnx_model = onnx.shape_inference.infer_shapes(onnx_model)
    onnx.save(onnx_model, output_path)
    
    print(f"ONNX model exported to {output_path}")
    
    # Verify model size
    size_bytes = os.path.getsize(output_path)
    print(f"Model size: {size_bytes} bytes ({size_bytes / 1024:.2f} KB)")
    
    return size_bytes


def quantize_model(onnx_path: str, output_path: str):
    """Quantize ONNX model to int8"""
    try:
        from onnxruntime.quantization import quantize_dynamic, QuantType
        
        quantize_dynamic(
            onnx_path,
            output_path,
            weight_type=QuantType.QInt8
        )
        
        size_bytes = os.path.getsize(output_path)
        print(f"Quantized model exported to {output_path}")
        print(f"Quantized model size: {size_bytes} bytes ({size_bytes / 1024:.2f} KB)")
        
        return size_bytes
    except ImportError:
        print("Warning: onnxruntime.quantization not available, skipping quantization")
        return None


def main():
    parser = argparse.ArgumentParser(description="Train resource predictor model")
    parser.add_argument("--data", type=str, default="data/training_data.npz", help="Training data path")
    parser.add_argument("--output", type=str, default="models/", help="Output directory")
    parser.add_argument("--epochs", type=int, default=50, help="Number of epochs")
    parser.add_argument("--batch-size", type=int, default=256, help="Batch size")
    parser.add_argument("--lr", type=float, default=0.001, help="Learning rate")
    parser.add_argument("--seed", type=int, default=42, help="Random seed")
    args = parser.parse_args()
    
    # Set seeds for reproducibility
    torch.manual_seed(args.seed)
    np.random.seed(args.seed)
    
    # Device selection
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    print(f"Using device: {device}")
    
    # Create output directory
    os.makedirs(args.output, exist_ok=True)
    
    # Load data
    print(f"Loading data from {args.data}...")
    train_loader, val_loader = load_data(args.data, args.batch_size)
    print(f"Training samples: {len(train_loader.dataset)}")
    print(f"Validation samples: {len(val_loader.dataset)}")
    
    # Create model
    model = ResourcePredictor().to(device)
    print(f"\nModel parameters: {model.count_parameters()}")
    
    # Loss and optimizer
    criterion = nn.MSELoss()
    optimizer = optim.Adam(model.parameters(), lr=args.lr)
    scheduler = optim.lr_scheduler.ReduceLROnPlateau(optimizer, patience=5, factor=0.5)
    
    # Training loop
    print(f"\nTraining for {args.epochs} epochs...")
    best_val_loss = float("inf")
    best_model_state = None
    
    for epoch in range(args.epochs):
        train_loss = train_epoch(model, train_loader, criterion, optimizer, device)
        val_loss = validate(model, val_loader, criterion, device)
        scheduler.step(val_loss)
        
        if val_loss < best_val_loss:
            best_val_loss = val_loss
            best_model_state = model.state_dict().copy()
        
        if (epoch + 1) % 10 == 0 or epoch == 0:
            print(f"Epoch {epoch + 1:3d}: train_loss={train_loss:.6f}, val_loss={val_loss:.6f}")
    
    # Load best model
    model.load_state_dict(best_model_state)
    print(f"\nBest validation loss: {best_val_loss:.6f}")
    
    # Save PyTorch model
    torch_path = os.path.join(args.output, "predictor.pt")
    torch.save({
        "model_state_dict": model.state_dict(),
        "model_version": "v1.0.0",
        "input_size": 12,
        "output_size": 5,
        "val_loss": best_val_loss
    }, torch_path)
    print(f"PyTorch model saved to {torch_path}")
    
    # Export to ONNX
    onnx_path = os.path.join(args.output, "predictor.onnx")
    export_onnx(model, onnx_path)
    
    # Quantize to int8
    quantized_path = os.path.join(args.output, "predictor_int8.onnx")
    quantize_model(onnx_path, quantized_path)
    
    print("\nTraining complete!")


if __name__ == "__main__":
    main()
