import numpy as np
import tensorflow as tf
from tensorflow.keras import layers, models, backend as K
import matplotlib.pyplot as plt
import mplfinance as mpf
import pandas as pd

# Hyperparameters
LATENT_DIM = 2
BATCH_SIZE = 64
LEARNING_RATE = 1e-3
EPOCHS = 50


# VAE Model
def sampling(args):
    z_mean, z_log_var = args
    batch = K.shape(z_mean)[0]
    dim = K.int_shape(z_mean)[1]
    epsilon = K.random_normal(shape=(batch, dim))
    return z_mean + K.exp(0.5 * z_log_var) * epsilon


def create_vae(input_shape):
    # Encoder
    inputs = layers.Input(shape=input_shape)
    h = layers.Conv1D(64, kernel_size=3, activation="relu", padding="same")(inputs)
    h = layers.Flatten()(h)
    h = layers.Dense(128, activation="relu")(h)
    z_mean = layers.Dense(LATENT_DIM)(h)
    z_log_var = layers.Dense(LATENT_DIM)(h)

    # Sampling layer
    z = layers.Lambda(sampling, output_shape=(LATENT_DIM,))([z_mean, z_log_var])

    # Decoder
    decoder_h = layers.Dense(input_shape[0] * 64, activation="relu")
    decoder_reshape = layers.Reshape((input_shape[0], 64))
    decoder_conv = layers.Conv1D(4, kernel_size=3, activation="tanh", padding="same")

    h_decoded = decoder_h(z)
    h_decoded = decoder_reshape(h_decoded)
    x_decoded = decoder_conv(h_decoded)

    # VAE Model
    vae = models.Model(inputs, x_decoded)

    # Loss function
    reconstruction_loss = tf.reduce_mean(tf.square(inputs - x_decoded))
    kl_loss = -0.5 * tf.reduce_mean(1 + z_log_var - K.square(z_mean) - K.exp(z_log_var))
    vae_loss = reconstruction_loss + kl_loss

    vae.add_loss(vae_loss)
    vae.compile(optimizer=tf.keras.optimizers.Adam(learning_rate=LEARNING_RATE))

    # Create a separate decoder model
    decoder_input = layers.Input(shape=(LATENT_DIM,))
    _h_decoded = decoder_h(decoder_input)
    _h_decoded = decoder_reshape(_h_decoded)
    _x_decoded = decoder_conv(_h_decoded)
    decoder_model = models.Model(decoder_input, _x_decoded)

    return vae, models.Model(inputs, z_mean), decoder_model


# Data Normalization
def normalize_data(data):
    return (data - np.mean(data, axis=0)) / np.std(data, axis=0)


# User Labeling
def user_label(data):
    df = pd.DataFrame(data, columns=["Open", "High", "Low", "Close"])
    df.index = pd.date_range(start="2023-01-01", periods=len(df), freq="min")
    fig, axlist = mpf.plot(df, type="candle", returnfig=True)
    plt.show(block=False)
    plt.pause(0.1)
    label = input("Enter label (1 for pattern, 0 for not a pattern): ")
    plt.close(fig)
    return int(label)


# Human-in-the-Loop Training
def human_in_the_loop_training(initial_data, vae, encoder, decoder, iterations=100):
    labeled_data = [
        (initial_data, 1)
    ]  # Start with the initial sample labeled as a pattern

    for i in range(iterations):
        # Prepare data for VAE training
        train_data = np.array([x for x, _ in labeled_data])
        train_data = train_data.reshape(
            -1, initial_data.shape[0], 4
        )  # Reshape to fit VAE input

        # Train VAE on the labeled data
        vae.fit(train_data, epochs=EPOCHS, batch_size=BATCH_SIZE, verbose=0)

        # Generate new samples from the latent space with small perturbations
        z_mean = encoder.predict(
            train_data
        )  # Use the encoder to get the latent representation
        new_sample = z_mean[0]  # Use the mean of the latent space
        perturbation = np.random.normal(0, 0.1, LATENT_DIM)  # Small random perturbation
        new_sample += perturbation
        generated_data = decoder.predict(np.array([new_sample]))[0]

        # User labels the new sample
        user_labeling = user_label(generated_data)
        labeled_data.append((generated_data, user_labeling))

        print(
            f"Iteration {i + 1}: Sample labeled as {'pattern' if user_labeling == 1 else 'not a pattern'}"
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
        [1635057600, 6.25, 6.38, 6.07, 6.1],
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

# Create VAE model
vae, encoder, decoder = create_vae(input_shape=(initial_data.shape[0], 4))

# Start the human-in-the-loop training
human_in_the_loop_training(initial_data, vae, encoder, decoder, iterations=100)
