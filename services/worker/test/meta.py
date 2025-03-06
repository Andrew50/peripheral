import numpy as np
from tensorflow.keras import backend as K
import tensorflow as tf
from tensorflow.keras import layers, models
from tensorflow.keras.models import Sequential
from tensorflow.keras.layers import Input
from tensorflow.keras.optimizers import Adam
import matplotlib.pyplot as plt
import mplfinance as mpf
import pandas as pd

# Hyperparameters
LATENT_DIM = 2
BATCH_SIZE = 64
LEARNING_RATE = 1e-3
EPOCHS = 50

CONV_FILTER_UNITS = [64]
CONV_FILTER_KERNAL_SIZES = [5]
BI_LSTM_UNITS = [64, 16]
DROPOUT_PERCENTS = [0.5]

# Define global parameters for adaptive exploration
success_threshold = 0.6  # Threshold for successful exploration
increase_factor = 1.2  # Factor to increase noise when successful
decay_factor = 0.8  # Factor to decrease noise when unsuccessful

def create_vae_and_decoder(input_shape=(30, 4), latent_dim=2):
    # Encoder
    inputs = layers.Input(shape=input_shape)
    h = layers.Conv1D(64, kernel_size=3, activation="relu", padding="same")(inputs)
    h = layers.Flatten()(h)
    h = layers.Dense(128, activation="relu")(h)
    z_mean = layers.Dense(latent_dim, name="z_mean")(h)
    z_log_var = layers.Dense(latent_dim, name="z_log_var")(h)

    # Sampling layer
    def sampling(args):
        z_mean, z_log_var = args
        batch = K.shape(z_mean)[0]
        dim = K.int_shape(z_mean)[1]
        epsilon = K.random_normal(shape=(batch, dim))
        return z_mean + K.exp(0.5 * z_log_var) * epsilon

    z = layers.Lambda(sampling, output_shape=(latent_dim,), name="z")(
        [z_mean, z_log_var]
    )

    # Decoder
    latent_inputs = layers.Input(shape=(latent_dim,), name="z_sampling")
    h_decoded = layers.Dense(128, activation="relu")(latent_inputs)
    h_decoded = layers.Dense(input_shape[0] * 64, activation="relu")(h_decoded)
    h_decoded = layers.Reshape((input_shape[0], 64))(h_decoded)
    x_decoded = layers.Conv1D(4, kernel_size=3, activation="tanh", padding="same")(
        h_decoded
    )

    # Instantiate decoder model
    decoder = models.Model(latent_inputs, x_decoded, name="decoder")

    # Apply the decoder to the sampled latent vector
    outputs = decoder(z)

    # Instantiate VAE model
    vae = models.Model(inputs, outputs, name="vae")

    # VAE loss function using Keras backend functions
    reconstruction_loss = K.mean(K.sum(K.square(inputs - outputs), axis=[1, 2]))
    kl_loss = -0.5 * K.mean(1 + z_log_var - K.square(z_mean) - K.exp(z_log_var))
    vae_loss = reconstruction_loss + kl_loss

    vae.add_loss(vae_loss)
    vae.compile(optimizer=Adam(learning_rate=LEARNING_RATE))

    # Encoder model
    encoder = models.Model(inputs, z_mean, name="encoder")

    return vae, encoder, decoder


def normalize_data(data):
    return (data - np.mean(data, axis=0)) / np.std(data, axis=0)


def select_uncertain_charts(classifier, generated_charts, max_samples=5):
    # Get predictions for each generated chart
    predictions = classifier.predict(generated_charts)

    # Calculate the uncertainty as the absolute distance from 0.5
    uncertainty_scores = np.abs(predictions - 0.5).flatten()

    # Get indices of charts with the smallest uncertainty scores
    uncertain_indexes = np.argsort(uncertainty_scores)[:max_samples]

    # Select the most uncertain charts to present to the user
    return [generated_charts[i] for i in uncertain_indexes]


def createModel():
    model = Sequential()
    model.add(Input(shape=(None, 4)))  # assuming o h l c
    for units, kernel_size in zip(CONV_FILTER_UNITS, CONV_FILTER_KERNAL_SIZES):
        model.add(
            layers.Conv1D(filters=units, kernel_size=kernel_size, activation="relu")
        )
    for units, dropout in zip(BI_LSTM_UNITS[:-1], DROPOUT_PERCENTS):
        model.add(layers.Bidirectional(layers.LSTM(units=units, return_sequences=True)))
        model.add(layers.Dropout(dropout))
    model.add(
        layers.Bidirectional(
            layers.LSTM(units=BI_LSTM_UNITS[-1], return_sequences=False)
        )
    )
    model.add(layers.Dense(1, activation="sigmoid"))
    model.compile(
        optimizer=Adam(learning_rate=LEARNING_RATE),
        loss="binary_crossentropy",
        metrics=[tf.keras.metrics.AUC(curve="PR", name="auc_pr")],
    )
    return model


def user_label(data):
    df = pd.DataFrame(data, columns=["Open", "High", "Low", "Close"])
    df.index = pd.date_range(start="2023-01-01", periods=len(df), freq="min")
    fig, axlist = mpf.plot(df, type="candle", returnfig=True)
    plt.show(block=False)
    plt.pause(0.1)
    label = input("Enter label (1 for pattern, 0 for not a pattern): ")
    plt.close(fig)
    return int(label)


def adaptive_exploration(
    initial_data, vae, encoder, decoder, classifier, iterations=100
):
    # Define exploration parameters
    success_threshold = 0.6  # Threshold for successful exploration
    increase_factor = 1.2  # Factor to increase noise when successful
    decay_factor = 0.8  # Factor to decrease noise when unsuccessful

    global current_noise_level, success_count, total_count

    labeled_data = [(initial_data, 1)]  # Initial sample labeled as an example

    for i in range(iterations):
        # Train VAE on the labeled data
        train_data = np.array([x for x, _ in labeled_data])
        vae.fit(train_data, epochs=5, batch_size=64, verbose=0)

        # Generate multiple samples in latent space
        z_mean = encoder.predict(train_data)[0]
        perturbations = np.random.normal(
            0, current_noise_level, (10, LATENT_DIM)
        )  # Generate 10 variations
        latent_vectors = z_mean + perturbations
        generated_samples = decoder.predict(latent_vectors)

        # Use the classifier to select the most uncertain charts
        uncertain_charts = select_uncertain_charts(
            classifier, generated_samples, max_samples=5
        )

        # User labels the selected uncertain charts
        for chart in uncertain_charts:
            user_labeling = user_label(chart)
            labeled_data.append((chart, user_labeling))

            # Track success rate
            if user_labeling == 1:
                success_count += 1
            total_count += 1

        # Calculate current success rate
        success_rate = success_count / total_count

        # Adjust exploration level based on success rate
        if success_rate >= success_threshold:
            current_noise_level *= increase_factor  # Explore further
        else:
            current_noise_level *= decay_factor  # Stay closer to initial input

        # Ensure noise level remains within a sensible range
        current_noise_level = max(0.001, min(current_noise_level, 0.1))

        print(
            f"Iteration {i + 1}: Success Rate = {success_rate:.2f}, Noise Level = {current_noise_level:.4f}"
        )


# Initial Data
data = np.array(
    [
        [1636699200, 7.96, 11.17, 7.92, 10.74],
        [1636612800, 6.76, 6.79, 6.4, 6.74],
        [1636526400, 6.85, 7.035, 6.77, 6.75],
        [1636440000, 6.52, 6.99, 6.46, 6.78],
        [1636353600, 6.08, 6.699, 6.19, 6.32],
        [1636267200, 6.21, 6.61, 6.1, 6.25],
        [1636180800, 6.02, 6.6, 6.01, 6.25],
        [1636094400, 6.48, 6.85, 6.32, 6.47],
        [1636008000, 6.05, 6.17, 5.95, 5.99],
        [1635921600, 6.01, 6.17, 5.95, 5.99],
        [1635835200, 6.14, 6.17, 6.05, 6.07],
        [1635748800, 6.01, 6.14, 6.03, 6.07],
        [1635662400, 6.16, 6.24, 6.02, 6.13],
        [1635576000, 6.23, 6.25, 6.1, 6.11],
        [1635489600, 6.07, 6.19, 5.98, 5.98],
        [1635403200, 6.15, 6.215, 6.05, 6.1],
        [1635316800, 6.02, 6.19, 6.01, 6.13],
        [1635230400, 6.02, 6.192, 6.01, 6.13],
        [1635144000, 6.25, 6.38, 6.07, 6.1],
        [1635058600, 6.25, 6.38, 6.07, 6.1],
        [1634971200, 6.66, 6.69, 6.43, 6.48],
        [1634884800, 6.64, 6.69, 6.43, 6.48],
        [1634798400, 6.77, 6.84, 6.58, 6.71],
        [1634712000, 6.76, 6.92, 6.7, 6.86],
        [1634625600, 6.76, 6.92, 6.7, 6.86],
        [1634539200, 6.38, 6.75, 6.28, 6.72],
        [1634452800, 6.25, 6.34, 6.05, 6.19],
        [1634366400, 6.25, 6.34, 6.05, 6.19],
        [1634280000, 6.61, 6.75, 6.61, 6.72],
        [1634193600, 6.53, 6.75, 6.28, 6.34],
    ]
)
initial_data = normalize_data(data[:, 1:])  # Use OHLC data and normalize

# Initialize classifier and VAE models
classifier = createModel()
vae, encoder, decoder = (
    create_vae_and_decoder()
)  # Define this function to create the VAE and decoder

# Start the human-in-the-loop training
adaptive_exploration(initial_data, vae, encoder, decoder, classifier, iterations=100)
