#!/usr/bin/env python3
"""
Training Script for Container Resource Predictor Model

Trains an LSTM-based neural network for resource prediction and exports to ONNX.
The LSTM captures temporal patterns in workload data for better predictions.
"""

import argparse
import os
import numpy as np
import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import DataLoader, TensorDataset


class LSTMResourcePredictor(nn.Module):
    """
    LSTM-based neural network for resource prediction.
    
    Architecture:
        Input(seq_len, 12) -> LSTM(hidden=64, layers=2) -> Dense(32) -> Output(5)
    
    Captures temporal patterns in workload metrics for better predictions.
    ~35,000 parameters, designed for <10ms inference on edge devices.
    """
    
    def __init__(
        self, 
        input_size: int = 12, 
        hidden_size: int = 64, 
        num_layers: int = 2,
        output_size: int = 5,
        dropout: float = 0.2
    ):
        super().__init__()
        self.hidden_size = hidden_size
        self.num_layers = num_layers
        
        # LSTM layer for sequence processing
        self.lstm = nn.LSTM(
            input_size=input_size,
            hidden_size=hidden_size,
            num_layers=num_layers,
            batch_first=True,
            dropout=dropout if num_layers > 1 else 0
        )
        
        # Fully connected layers for prediction
        self.fc = nn.Sequential(
            nn.Linear(hidden_size, 32),
            nn.ReLU(),
            nn.Dropout(dropout),
            nn.Linear(32, output_size),
            nn.Sigmoid()  # Output normalized 0-1
        )
    
    def forward(self, x: torch.Tensor) -> torch.Tensor:
        # x shape: (batch, seq_len, features)
        lstm_out, (h_n, c_n) = self.lstm(x)
        
        # Use the last hidden state for prediction
        last_hidden = h_n[-1]  # (batch, hidden_size)
        
        # Pass through fully connected layers
        output = self.fc(last_hidden)
        return output
    
    def count_parameters(self) -> int:
        return sum(p.numel() for p in self.parameters() if p.requires_grad)


def load_sequence_data(data_path: str, batch_size: int = 64):
    """Load sequence training data from npz file"""
    data = np.load(data_path)
    
    # Load sequence data for LSTM
    train_sequences = torch.FloatTensor(data["train_sequences"])
    train_labels = torch.FloatTensor(data["train_labels"])
    val_sequences = torch.FloatTensor(data["val_sequences"])
    val_labels = torch.FloatTensor(data["val_labels"])
    
    train_dataset = TensorDataset(train_sequences, train_labels)
    val_dataset = TensorDataset(val_sequences, val_labels)
    
    train_loader = DataLoader(train_dataset, batch_size=batch_size, shuffle=True)
    val_loader = DataLoader(val_dataset, batch_size=batch_size, shuffle=False)
    
    return train_loader, val_loader


def train_epoch(model, train_loader, criterion, optimizer, device):
    """Train for one epoch"""
    model.train()
    total_loss = 0.0
    
    for sequences, labels in train_loader:
        sequences, labels = sequences.to(device), labels.to(device)
        
        optimizer.zero_grad()
        outputs = model(sequences)
        loss = criterion(outputs, labels)
        loss.backward()
        
        # Gradient clipping for LSTM stability
        torch.nn.utils.clip_grad_norm_(model.parameters(), max_norm=1.0)
        
        optimizer.step()
        
        total_loss += loss.item() * sequences.size(0)
    
    return total_loss / len(train_loader.dataset)


def validate(model, val_loader, criterion, device):
    """Validate model"""
    model.eval()
    total_loss = 0.0
    
    with torch.no_grad():
        for sequences, labels in val_loader:
            sequences, labels = sequences.to(device), labels.to(device)
            outputs = model(sequences)
            loss = criterion(outputs, labels)
            total_loss += loss.item() * sequences.size(0)
    
    return total_loss / len(val_loader.dataset)


def export_onnx(model, output_path: str, seq_len: int = 10, input_size: int = 12):
    """Export LSTM model to ONNX format"""
    import onnx
    
    model.eval()
    # LSTM expects (batch, seq_len, features)
    dummy_input = torch.randn(1, seq_len, input_size)
    
    torch.onnx.export(
        model,
        dummy_input,
        output_path,
        export_params=True,
        opset_version=17,
        do_constant_folding=True,
        input_names=["sequence"],
        output_names=["predictions"],
        dynamic_axes={
            "sequence": {0: "batch_size"},
            "predictions": {0: "batch_size"}
        }
    )
    
    # Run shape inference
    onnx_model = onnx.load(output_path)
    onnx_model = onnx.shape_inference.infer_shapes(onnx_model)
    onnx.save(onnx_model, output_path)
    
    print(f"ONNX model exported to {output_path}")
    
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
    parser = argparse.ArgumentParser(description="Train LSTM resource predictor model")
    parser.add_argument("--data", type=str, default="data/training_data.npz", help="Training data path")
    parser.add_argument("--output", type=str, default="models/", help="Output directory")
    parser.add_argument("--epochs", type=int, default=100, help="Number of epochs")
    parser.add_argument("--batch-size", type=int, default=64, help="Batch size")
    parser.add_argument("--lr", type=float, default=0.001, help="Learning rate")
    parser.add_argument("--hidden-size", type=int, default=64, help="LSTM hidden size")
    parser.add_argument("--num-layers", type=int, default=2, help="Number of LSTM layers")
    parser.add_argument("--seq-len", type=int, default=10, help="Sequence length")
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
    train_loader, val_loader = load_sequence_data(args.data, args.batch_size)
    print(f"Training samples: {len(train_loader.dataset)}")
    print(f"Validation samples: {len(val_loader.dataset)}")
    
    # Create LSTM model
    model = LSTMResourcePredictor(
        input_size=12,
        hidden_size=args.hidden_size,
        num_layers=args.num_layers,
        output_size=5
    ).to(device)
    
    print(f"\nModel: LSTM Resource Predictor")
    print(f"Parameters: {model.count_parameters():,}")
    print(f"Hidden size: {args.hidden_size}")
    print(f"LSTM layers: {args.num_layers}")
    print(f"Sequence length: {args.seq_len}")
    
    # Loss and optimizer
    criterion = nn.MSELoss()
    optimizer = optim.Adam(model.parameters(), lr=args.lr, weight_decay=1e-5)
    scheduler = optim.lr_scheduler.ReduceLROnPlateau(
        optimizer, mode='min', patience=10, factor=0.5
    )
    
    # Training loop with early stopping
    print(f"\nTraining for up to {args.epochs} epochs...")
    best_val_loss = float("inf")
    best_model_state = None
    patience_counter = 0
    early_stop_patience = 20
    
    for epoch in range(args.epochs):
        train_loss = train_epoch(model, train_loader, criterion, optimizer, device)
        val_loss = validate(model, val_loader, criterion, device)
        scheduler.step(val_loss)
        
        if val_loss < best_val_loss:
            best_val_loss = val_loss
            best_model_state = model.state_dict().copy()
            patience_counter = 0
        else:
            patience_counter += 1
        
        if (epoch + 1) % 10 == 0 or epoch == 0:
            print(f"Epoch {epoch + 1:3d}: train_loss={train_loss:.6f}, val_loss={val_loss:.6f}")
        
        # Early stopping
        if patience_counter >= early_stop_patience:
            print(f"\nEarly stopping at epoch {epoch + 1}")
            break
    
    # Load best model
    model.load_state_dict(best_model_state)
    print(f"\nBest validation loss: {best_val_loss:.6f}")
    
    # Save PyTorch model
    torch_path = os.path.join(args.output, "predictor_lstm.pt")
    torch.save({
        "model_state_dict": model.state_dict(),
        "model_type": "lstm",
        "model_version": "v1.0.0",
        "input_size": 12,
        "output_size": 5,
        "hidden_size": args.hidden_size,
        "num_layers": args.num_layers,
        "seq_len": args.seq_len,
        "val_loss": best_val_loss
    }, torch_path)
    print(f"PyTorch model saved to {torch_path}")
    
    # Export to ONNX
    onnx_path = os.path.join(args.output, "predictor_lstm.onnx")
    export_onnx(model, onnx_path, seq_len=args.seq_len)
    
    # Quantize to int8
    quantized_path = os.path.join(args.output, "predictor_lstm_int8.onnx")
    quantize_model(onnx_path, quantized_path)
    
    print("\n" + "="*50)
    print("Training complete!")
    print("="*50)
    print(f"Model: LSTM ({args.num_layers} layers, {args.hidden_size} hidden)")
    print(f"Best validation loss: {best_val_loss:.6f}")
    print(f"Output files:")
    print(f"  - {torch_path}")
    print(f"  - {onnx_path}")
    print(f"  - {quantized_path}")


if __name__ == "__main__":
    main()
