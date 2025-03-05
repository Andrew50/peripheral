import numpy as np
import tensorflow as tf
import matplotlib.pyplot as plt
import mplfinance as mpf
import pandas as pd
from tensorflow.keras import layers, models
from tensorflow.keras.models import Model
from tensorflow.keras.layers import (
    Dense,
    LSTM,
    Bidirectional,
    Dropout,
    Conv1D,
    Flatten,
    Lambda,
    Input,
    Reshape,
)
from tensorflow.keras.optimizers import Adam
from tensorflow.keras.callbacks import EarlyStopping
from tensorflow.keras import backend as K

SPLIT_RATIO = 0.8
TRAINING_CLASS_RATIO = 0.18
VALIDATION_CLASS_RATIO = 0.065
MIN_RANDOM_NOS = 0.7
MAX_EPOCHS = 2000
PATIENCE_EPOCHS = 150
MIN_EPOCHS = 50
BATCH_SIZE = 64
CONV_FILTER_UNITS = [64]
CONV_FILTER_KERNAL_SIZES = [5]
BI_LSTM_UNITS = [64, 16]
DROPOUT_PERCENTS = [0.5]
LATENT_DIM = 10  # Reduced latent dimension for tighter initial fitting
INITIAL_KL_WEIGHT = 0.0  # High initial KL weight for warm-up
KL_DECAY_RATE = 0.99  # Decay rate of KL weight per epoch
MAX_KL_WEIGHT = 1.0  # Maximum value for KL weight
KL_STEP = 0.001  # Incremental increase per epoch


def augment_data(data, noise_level=0.01):
    noise = np.random.normal(scale=noise_level, size=data.shape)
    return data + noise


def createModel():
    model = Sequential()
    model.add(Input(shape=(None, 4)))  # assuming o h l c
    for units, kernal_size in zip(CONV_FILTER_UNITS, CONV_FILTER_KERNAL_SIZES):
        model.add(Conv1D(filters=units, kernel_size=kernal_size, activation="tanh"))
    for units, dropout in zip(BI_LSTM_UNITS[:-1], DROPOUT_PERCENTS):
        model.add(Bidirectional(LSTM(units=units, return_sequences=True)))
        model.add(Dropout(dropout))
    model.add(Bidirectional(LSTM(units=BI_LSTM_UNITS[-1], return_sequences=False)))
    model.add(Dense(1, activation="sigmoid"))
    model.compile(
        optimizer=Adam(learning_rate=1e-3),
        loss="binary_crossentropy",
        metrics=[tf.keras.metrics.AUC(curve="PR", name="auc_pr")],
    )
    return model


def sampling(args):
    z_mean, z_log_var = args
    epsilon = K.random_normal(shape=K.shape(z_mean), mean=0.0, stddev=1.0)
    return z_mean + K.exp(0.5 * z_log_var) * epsilon


def build_encoder():
    inputs = Input(shape=(30, 4))
    labels = Input(shape=(1,), name="labels")  # Add label input
    label_embedding = layers.Embedding(input_dim=2, output_dim=4)(
        labels
    )  # Assuming binary labels (0, 1)
    label_embedding = layers.Flatten()(label_embedding)
    label_embedding = layers.RepeatVector(30)(
        label_embedding
    )  # Repeat to match input sequence length

    x = layers.Concatenate()(
        [inputs, label_embedding]
    )  # Concatenate label with input data
    for units, kernal_size in zip(CONV_FILTER_UNITS, CONV_FILTER_KERNAL_SIZES):
        x = Conv1D(
            filters=units, kernel_size=kernal_size, activation="tanh", padding="same"
        )(x)
    for units in BI_LSTM_UNITS:
        x = Bidirectional(LSTM(units=units, return_sequences=True))(x)
    x = Flatten()(x)
    x = Dense(128, activation="tanh")(x)
    z_mean = Dense(LATENT_DIM, name="z_mean")(x)
    z_log_var = Dense(LATENT_DIM, name="z_log_var")(x)
    z = Lambda(sampling)([z_mean, z_log_var])
    encoder = Model(
        [inputs, labels], [z_mean, z_log_var, z], name="encoder"
    )  # Note: now takes labels as input
    return encoder


def build_decoder():
    latent_inputs = Input(shape=(LATENT_DIM,))
    labels = Input(shape=(1,), name="labels")  # Add label input to decoder
    label_embedding = layers.Embedding(input_dim=2, output_dim=LATENT_DIM)(labels)
    label_embedding = layers.Flatten()(label_embedding)
    latent_with_labels = layers.Concatenate()([latent_inputs, label_embedding])

    x = Dense(128, activation="tanh")(latent_with_labels)
    x = Dense(30 * 4, activation="tanh")(x)
    x = Reshape((30, 4))(x)
    for units in reversed(BI_LSTM_UNITS):
        x = Bidirectional(LSTM(units=units, return_sequences=True))(x)
    for units, kernal_size in zip(
        reversed(CONV_FILTER_UNITS), reversed(CONV_FILTER_KERNAL_SIZES)
    ):
        x = Conv1D(
            filters=units, kernel_size=kernal_size, activation="tanh", padding="same"
        )(x)
    outputs = Conv1D(filters=4, kernel_size=1, activation="tanh", padding="same")(x)
    decoder = Model(
        [latent_inputs, labels], outputs, name="decoder"
    )  # Note: now takes labels as input
    return decoder


# Define the VAE as a subclassed Model
class VAE(tf.keras.Model):
    def __init__(self, encoder, decoder, **kwargs):
        super(VAE, self).__init__(**kwargs)
        self.encoder = encoder
        self.decoder = decoder
        self.kl_weight = INITIAL_KL_WEIGHT

    def call(self, inputs):
        inputs, labels = inputs  # Unpack inputs and labels
        z_mean, z_log_var, z = self.encoder([inputs, labels])
        reconstructed = self.decoder([z, labels])
        return reconstructed

    def train_step(self, data):
        (inputs, labels), targets = data
        with tf.GradientTape() as tape:
            z_mean, z_log_var, z = self.encoder([inputs, labels])
            reconstruction = self.decoder([z, labels])

            # Compute sample weights
            sample_weights = tf.where(
                tf.equal(labels, 1), 2.0, 1.0
            )  # Weight 'yes' samples more
            sample_weights = tf.cast(sample_weights, tf.float32)

            # Compute reconstruction loss with sample weights
            reconstruction_loss = tf.reduce_mean(
                sample_weights
                * tf.reduce_sum(tf.square(targets - reconstruction), axis=[1, 2])
            )

            # Compute KL divergence loss
            kl_loss = -0.5 * tf.reduce_mean(
                tf.reduce_sum(
                    1 + z_log_var - tf.square(z_mean) - tf.exp(z_log_var), axis=1
                )
            )

            # Total loss
            total_loss = reconstruction_loss + self.kl_weight * kl_loss

        # Apply gradients
        grads = tape.gradient(total_loss, self.trainable_variables)
        self.optimizer.apply_gradients(zip(grads, self.trainable_variables))

        # Update KL weight
        self.kl_weight = min(MAX_KL_WEIGHT, self.kl_weight + KL_STEP)

        return {
            "loss": total_loss,
            "reconstruction_loss": reconstruction_loss,
            "kl_loss": kl_loss,
        }


def user_label(data):
    # Assuming `data` is in the shape (sequence_length, 4)
    df = pd.DataFrame(data, columns=["Open", "High", "Low", "Close"])
    df.index = pd.date_range(
        start="2023-01-01", periods=len(df), freq="min"
    )  # Create a dummy time index

    # Plot the chart using mplfinance
    fig, axlist = mpf.plot(df, type="candle", returnfig=True)

    # Variable to store the label
    label = None

    # Event handler for mouse clicks
    def on_click(event):
        nonlocal label
        if event.button == 1:  # Left click for "pattern"
            label = 1
            print("Left click detected: Labeled as pattern (1)")
        elif event.button == 3:  # Right click for "not a pattern"
            label = 0
            print("Right click detected: Labeled as not a pattern (0)")
        else:
            raise Exception()
        plt.close(fig)  # Close the plot after a click

    # Connect the click event to the event handler
    fig.canvas.mpl_connect("button_press_event", on_click)

    # Show the plot and wait for the user's input
    plt.show()

    # Return the captured label
    return label


def select_uncertain_charts(classifier, generated_charts, max_samples=5):
    # Get predictions for each generated chart
    predictions = classifier.predict(generated_charts)

    # Calculate the uncertainty as the absolute distance from 0.5
    uncertainty_scores = np.abs(predictions - 0.5).flatten()

    # Get indices of charts with the smallest uncertainty scores
    uncertain_indexes = np.argsort(uncertainty_scores)[:max_samples]

    # Select the most uncertain charts to present to the user
    return [generated_charts[i] for i in uncertain_indexes]


def iterative_training_loop_with_classifier_guidance(
    data, decoder, classifier, vae, iterations=100
):
    labeled_data = []  # To store generated charts and user labels
    for i in range(iterations):
        # Generate latent samples with correct dimension
        latent_samples = np.random.normal(size=(10, LATENT_DIM))
        labels_for_generation = np.ones((10, 1), dtype=np.int32)
        generated_charts = decoder.predict([latent_samples, labels_for_generation])
        uncertain_charts = select_uncertain_charts(classifier, generated_charts)

        for chart in uncertain_charts:
            user_labeling = user_label(chart)  # Plot the chart and get the user's input
            labeled_data.append((chart, user_labeling))
        if len(labeled_data) >= 10:  # Retrain in batches
            charts, labels = zip(*labeled_data)
            charts = np.array(charts)  # Convert to numpy array
            labels = np.array(labels).astype(np.int32).reshape(-1, 1)
            assert np.all((labels == 0) | (labels == 1)), "Labels must be 0 or 1."
            classifier.fit(charts, labels, epochs=1, batch_size=5, verbose=0)
            vae.fit(x=(charts, labels), y=charts, epochs=100, batch_size=BATCH_SIZE)
            labeled_data.clear()

        print(
            f"Iteration {i + 1}: {len(uncertain_charts)} charts labeled and used for training"
        )


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


def normalize_data(data):
    # Min-Max normalization to the range [-1, 1] using global min and max
    global_min = np.min(data)
    global_max = np.max(data)
    normalized_data = 2 * (data - global_min) / (global_max - global_min) - 1
    return normalized_data


# Normalize the initial training data
initial_data = data[:, 1:][::-1]  # Extract OHLC columns and reverse the data
normalized_initial_data = normalize_data(initial_data)

# Augment the data
augmented_data_list = [augment_data(normalized_initial_data) for _ in range(10)]

# Label the augmented data using user input
labeled_data = []
for aug_data in augmented_data_list:
    # Reshape to fit the input format for `user_label`
    input_data = aug_data[:30].reshape(1, 30, 4)
    label = user_label(input_data[0])  # Pass the first (and only) example in the batch
    labeled_data.append((input_data, label))

# Prepare training data including both positive and negative examples
training_data = np.array(
    [data for data, label in labeled_data]
)  # Combine all input data, retaining the batch dimension
labels = np.array([label for _, label in labeled_data])  # Store labels as a numpy array

# Instantiate the models
# Prepare training data including both positive and negative examples
training_data = np.array(
    [data for data, label in labeled_data]
)  # Combine all input data, retaining the batch dimension
training_data = np.squeeze(
    training_data
)  # Remove any extra dimensions to get (batch_size, 30, 4)

labels = np.array([label for _, label in labeled_data])  # Store labels as a numpy array
labels = labels.reshape(-1, 1)  # Reshape labels to have shape (batch_size, 1)

# Instantiate the models
encoder = build_encoder()
decoder = build_decoder()
vae = VAE(encoder, decoder)
vae.compile(optimizer=Adam(learning_rate=1e-3))
classifier = createModel()

# Ensure the training data shape is correct (batch_size, 30, 4)
print(
    f"Training data shape: {training_data.shape}"
)  # Should print: (number_of_samples, 30, 4)
print(f"Labels shape: {labels.shape}")  # Should print: (number_of_samples, 1)

# Train the VAE on both positive and negative labeled data
vae.fit(x=(training_data, labels), y=training_data, epochs=50, batch_size=BATCH_SIZE)


# Start the iterative training loop
iterative_training_loop_with_classifier_guidance(
    data, decoder, classifier, vae, iterations=100000
)
