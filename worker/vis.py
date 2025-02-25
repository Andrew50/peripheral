import tensorflow as tf
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns

# Specify the path to the SavedModel directory
model_path = "./2"  # Update this to the correct path

# Load the SavedModel using TensorFlow's load function
loaded_model = tf.saved_model.load(model_path)

# To visualize the activations, we'll need to wrap the loaded model in a Keras model
# This requires knowing the input and output layers
# If you have access to the original model architecture, you can recreate it and load the weights


# Create a Keras Sequential model with the same architecture
def create_model_architecture():
    model = tf.keras.Sequential()
    model.add(
        tf.keras.layers.InputLayer(input_shape=(None, 4))
    )  # Assuming (None, 4) shape for OHLC data
    model.add(tf.keras.layers.Conv1D(filters=32, kernel_size=5, activation="tanh"))
    model.add(tf.keras.layers.Conv1D(filters=64, kernel_size=5, activation="tanh"))
    model.add(tf.keras.layers.LayerNormalization())
    model.add(
        tf.keras.layers.Bidirectional(
            tf.keras.layers.LSTM(units=64, return_sequences=False, dropout=0.65)
        )
    )
    model.add(tf.keras.layers.Dense(1, activation="sigmoid"))
    return model


# Recreate the model architecture
model = create_model_architecture()

# Load the weights from the SavedModel into the Keras model
for layer, loaded_layer in zip(
    model.layers, loaded_model.signatures["serving_default"].structured_outputs.values()
):
    if isinstance(layer, tf.keras.layers.Layer) and hasattr(layer, "set_weights"):
        layer.set_weights(loaded_layer)

# Generate random input matching the model's expected input shape
input_shape = (1, 30, 4)  # Update this to the actual input shape
random_input = np.random.random(input_shape)

# Create a new model that outputs activations for each layer
layer_outputs = [layer.output for layer in model.layers]
activation_model = tf.keras.models.Model(inputs=model.input, outputs=layer_outputs)

# Get activations for the random input
activations = activation_model.predict(random_input)

# Visualize the activations
for i, activation in enumerate(activations):
    if len(activation.shape) == 3:
        avg_activation = np.mean(activation, axis=0)
        plt.figure(figsize=(12, 6))
        sns.heatmap(avg_activation, cmap="viridis")
        plt.title(f"Layer {model.layers[i].name} - Average Activations")
        plt.xlabel("Features")
        plt.ylabel("Timesteps")
        plt.show()
    elif len(activation.shape) == 2:
        avg_activation = np.mean(activation, axis=0)
        plt.figure(figsize=(12, 6))
        plt.bar(range(len(avg_activation)), avg_activation)
        plt.title(f"Layer {model.layers[i].name} - Average Activations")
        plt.xlabel("Features")
        plt.ylabel("Activation")
        plt.show()
