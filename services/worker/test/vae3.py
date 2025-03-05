import numpy as np
import tensorflow as tf
import matplotlib.pyplot as plt
import mplfinance as mpf
import pandas as pd
from tensorflow.keras import layers
from tensorflow.keras.models import Model, Sequential
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
from tensorflow.keras import backend as K

# Hyperparameters
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
INITIAL_KL_WEIGHT = 0.0  # Start with zero KL weight
KL_DECAY_RATE = 0.99
MAX_KL_WEIGHT = 1.0  # Maximum value for KL weight
KL_STEP = 0.001  # Incremental increase per epoch


def augment_data(data, noise_level=0.01):
    noise = np.random.normal(scale=noise_level, size=data.shape)
    return data + noise


def normalize_data(data):
    # Min-Max normalization to the range [-1, 1]
    global_min = np.min(data)
    global_max = np.max(data)
    normalized_data = 2 * (data - global_min) / (global_max - global_min) - 1
    return normalized_data, global_min, global_max


def denormalize_data(data, global_min, global_max):
    denormalized_data = (data + 1) * (global_max - global_min) / 2 + global_min
    return denormalized_data


def sampling(args):
    z_mean, z_log_var = args
    epsilon = K.random_normal(shape=tf.shape(z_mean), mean=0.0, stddev=1.0)
    return z_mean + tf.exp(0.5 * z_log_var) * epsilon


# Build the unconditional encoder for pre-training
def build_unconditional_encoder():
    inputs = Input(shape=(30, 4))
    x = inputs
    for units, kernel_size in zip(CONV_FILTER_UNITS, CONV_FILTER_KERNAL_SIZES):
        x = Conv1D(
            filters=units, kernel_size=kernel_size, activation="tanh", padding="same"
        )(x)
    for units in BI_LSTM_UNITS:
        x = Bidirectional(LSTM(units=units, return_sequences=True))(x)
    x = Flatten()(x)
    x = Dense(128, activation="tanh")(x)
    z_mean = Dense(LATENT_DIM, name="z_mean")(x)
    z_log_var = Dense(LATENT_DIM, name="z_log_var")(x)
    z = Lambda(sampling)([z_mean, z_log_var])
    encoder = Model(inputs, [z_mean, z_log_var, z], name="unconditional_encoder")
    return encoder


# Build the unconditional decoder for pre-training
def build_unconditional_decoder():
    latent_inputs = Input(shape=(LATENT_DIM,))
    x = Dense(128, activation="tanh")(latent_inputs)
    x = Dense(30 * 4, activation="tanh")(x)
    x = Reshape((30, 4))(x)
    for units in reversed(BI_LSTM_UNITS):
        x = Bidirectional(LSTM(units=units, return_sequences=True))(x)
    for units, kernel_size in zip(
        reversed(CONV_FILTER_UNITS), reversed(CONV_FILTER_KERNAL_SIZES)
    ):
        x = Conv1D(
            filters=units, kernel_size=kernel_size, activation="tanh", padding="same"
        )(x)
    outputs = Conv1D(filters=4, kernel_size=1, activation="tanh", padding="same")(x)
    decoder = Model(latent_inputs, outputs, name="unconditional_decoder")
    return decoder


# Unconditional VAE for pre-training
class UnconditionalVAE(tf.keras.Model):
    def __init__(self, encoder, decoder, **kwargs):
        super(UnconditionalVAE, self).__init__(**kwargs)
        self.encoder = encoder
        self.decoder = decoder
        self.kl_weight = INITIAL_KL_WEIGHT

    def call(self, inputs):
        z_mean, z_log_var, z = self.encoder(inputs)
        reconstructed = self.decoder(z)
        return reconstructed

    def train_step(self, data):
        inputs = data
        with tf.GradientTape() as tape:
            z_mean, z_log_var, z = self.encoder(inputs)
            reconstruction = self.decoder(z)
            reconstruction_loss = tf.reduce_mean(
                tf.reduce_sum(tf.square(inputs - reconstruction), axis=[1, 2])
            )
            kl_loss = -0.5 * tf.reduce_mean(
                tf.reduce_sum(
                    1 + z_log_var - tf.square(z_mean) - tf.exp(z_log_var), axis=1
                )
            )
            total_loss = reconstruction_loss + self.kl_weight * kl_loss
        grads = tape.gradient(total_loss, self.trainable_variables)
        self.optimizer.apply_gradients(zip(grads, self.trainable_variables))
        self.kl_weight = min(MAX_KL_WEIGHT, self.kl_weight + KL_STEP)
        return {
            "loss": total_loss,
            "reconstruction_loss": reconstruction_loss,
            "kl_loss": kl_loss,
        }


# Build the conditional encoder for fine-tuning
def build_encoder():
    inputs = Input(shape=(30, 4))
    labels = Input(shape=(1,), name="labels")
    label_embedding = layers.Embedding(input_dim=2, output_dim=4)(labels)
    label_embedding = layers.Flatten()(label_embedding)
    label_embedding = layers.RepeatVector(30)(label_embedding)
    x = layers.Concatenate()([inputs, label_embedding])
    for units, kernel_size in zip(CONV_FILTER_UNITS, CONV_FILTER_KERNAL_SIZES):
        x = Conv1D(
            filters=units, kernel_size=kernel_size, activation="tanh", padding="same"
        )(x)
    for units in BI_LSTM_UNITS:
        x = Bidirectional(LSTM(units=units, return_sequences=True))(x)
    x = Flatten()(x)
    x = Dense(128, activation="tanh")(x)
    z_mean = Dense(LATENT_DIM, name="z_mean")(x)
    z_log_var = Dense(LATENT_DIM, name="z_log_var")(x)
    z = Lambda(sampling)([z_mean, z_log_var])
    encoder = Model([inputs, labels], [z_mean, z_log_var, z], name="encoder")
    return encoder


# Build the conditional decoder for fine-tuning
def build_decoder():
    latent_inputs = Input(shape=(LATENT_DIM,))
    labels = Input(shape=(1,), name="labels")
    label_embedding = layers.Embedding(input_dim=2, output_dim=LATENT_DIM)(labels)
    label_embedding = layers.Flatten()(label_embedding)
    latent_with_labels = layers.Concatenate()([latent_inputs, label_embedding])
    x = Dense(128, activation="tanh")(latent_with_labels)
    x = Dense(30 * 4, activation="tanh")(x)
    x = Reshape((30, 4))(x)
    for units in reversed(BI_LSTM_UNITS):
        x = Bidirectional(LSTM(units=units, return_sequences=True))(x)
    for units, kernel_size in zip(
        reversed(CONV_FILTER_UNITS), reversed(CONV_FILTER_KERNAL_SIZES)
    ):
        x = Conv1D(
            filters=units, kernel_size=kernel_size, activation="tanh", padding="same"
        )(x)
    outputs = Conv1D(filters=4, kernel_size=1, activation="tanh", padding="same")(x)
    decoder = Model([latent_inputs, labels], outputs, name="decoder")
    return decoder


# Conditional VAE for fine-tuning
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
            positive_weight = 2.0  # Adjust as needed
            sample_weights = tf.where(tf.equal(labels, 1), positive_weight, 1.0)
            sample_weights = tf.cast(sample_weights, tf.float32)

            # Compute reconstruction loss with sample weights
            reconstruction_loss = tf.reduce_mean(
                sample_weights
                * tf.reduce_sum(tf.square(targets - reconstruction), axis=[1, 2])
            )
            kl_loss = -0.5 * tf.reduce_mean(
                tf.reduce_sum(
                    1 + z_log_var - tf.square(z_mean) - tf.exp(z_log_var), axis=1
                )
            )
            total_loss = reconstruction_loss + self.kl_weight * kl_loss
        grads = tape.gradient(total_loss, self.trainable_variables)
        self.optimizer.apply_gradients(zip(grads, self.trainable_variables))
        self.kl_weight = min(MAX_KL_WEIGHT, self.kl_weight + KL_STEP)
        return {
            "loss": total_loss,
            "reconstruction_loss": reconstruction_loss,
            "kl_loss": kl_loss,
        }


def user_label(data):
    df = pd.DataFrame(data, columns=["Open", "High", "Low", "Close"])
    df.index = pd.date_range(
        start="2023-01-01", periods=len(df), freq="min"
    )  # Create a dummy time index
    fig, axlist = mpf.plot(df, type="candle", returnfig=True)
    label = None

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

    fig.canvas.mpl_connect("button_press_event", on_click)
    plt.show()
    return label


def select_uncertain_charts(classifier, generated_charts, max_samples=5):
    predictions = classifier.predict(generated_charts)
    uncertainty_scores = np.abs(predictions - 0.5).flatten()
    uncertain_indexes = np.argsort(uncertainty_scores)[:max_samples]
    return [generated_charts[i] for i in uncertain_indexes]


def create_classifier():
    model = Sequential()
    model.add(Input(shape=(30, 4)))
    for units, kernel_size in zip(CONV_FILTER_UNITS, CONV_FILTER_KERNAL_SIZES):
        model.add(Conv1D(filters=units, kernel_size=kernel_size, activation="tanh"))
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


def iterative_training_loop_with_classifier_guidance(
    decoder, classifier, vae, iterations=100
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
            vae.fit(x=(charts, labels), y=charts, epochs=20, batch_size=BATCH_SIZE)
            labeled_data.clear()
        print(
            f"Iteration {i + 1}: {len(uncertain_charts)} charts labeled and used for training"
        )


# Prepare random stock data for pre-training
# Assuming 'random_stock_data' is a NumPy array of shape (num_samples, 30, 4)
# For demonstration, we'll use the provided data and augment it
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

# Normalize the data
initial_data = data[:, 1:][::-1]  # Extract OHLC columns and reverse the data
normalized_initial_data, global_min, global_max = normalize_data(initial_data)

# Augment the data to create a larger dataset
random_stock_data = np.array(
    [augment_data(normalized_initial_data) for _ in range(1000)]
)
random_stock_data = random_stock_data.reshape(-1, 30, 4)
from train import getSample
from data import getTensor
from conn import Conn

conn = Conn(False)
setupID = 2
interval = "1d"
bars = 30

trainingSample, validationSample = getSample(conn, setupID, interval, 0.08, 0.8, 0.98)
# print(trainingSample)
xTrainingData, yTrainingData = getTensor(
    conn, trainingSample, interval, bars, normalize="min-max"
)
random_stock_data = xTrainingData

# Pre-train the unconditional VAE
unconditional_encoder = build_unconditional_encoder()
unconditional_decoder = build_unconditional_decoder()
unconditional_vae = UnconditionalVAE(unconditional_encoder, unconditional_decoder)
unconditional_vae.compile(optimizer=Adam(learning_rate=1e-3))
unconditional_vae.fit(x=random_stock_data, epochs=5, batch_size=BATCH_SIZE)

# Build the conditional VAE and transfer weights
encoder = build_encoder()
decoder = build_decoder()
vae = VAE(encoder, decoder)
vae.compile(optimizer=Adam(learning_rate=1e-4))

# Transfer weights from unconditional VAE to conditional VAE
# For encoder
# Transfer weights from unconditional VAE to conditional VAE

# For encoder
unconditional_encoder_layers = {
    layer.name: layer for layer in unconditional_encoder.layers
}
conditional_encoder_layers = {layer.name: layer for layer in encoder.layers}

for layer_name in unconditional_encoder_layers:
    if layer_name in conditional_encoder_layers:
        layer_uncond = unconditional_encoder_layers[layer_name]
        layer_cond = conditional_encoder_layers[layer_name]

        # Exclude label-related layers and layers without weights
        if (
            "labels" not in layer_name
            and "embedding" not in layer_name
            and layer_uncond.get_weights()
        ):
            try:
                layer_cond.set_weights(layer_uncond.get_weights())
                print(f"Transferred weights for layer: {layer_name}")
            except Exception as e:
                print(f"Could not transfer weights for layer: {layer_name}. Error: {e}")

# For decoder
unconditional_decoder_layers = {
    layer.name: layer for layer in unconditional_decoder.layers
}
conditional_decoder_layers = {layer.name: layer for layer in decoder.layers}

for layer_name in unconditional_decoder_layers:
    if layer_name in conditional_decoder_layers:
        layer_uncond = unconditional_decoder_layers[layer_name]
        layer_cond = conditional_decoder_layers[layer_name]

        # Exclude label-related layers and layers without weights
        if (
            "labels" not in layer_name
            and "embedding" not in layer_name
            and layer_uncond.get_weights()
        ):
            try:
                layer_cond.set_weights(layer_uncond.get_weights())
                print(f"Transferred weights for layer: {layer_name}")
            except Exception as e:
                print(f"Could not transfer weights for layer: {layer_name}. Error: {e}")


# Create the classifier
classifier = create_classifier()

# Collect initial user-labeled data
# For demonstration, we use the augmented data and simulate user labels
augmented_data_list = [augment_data(normalized_initial_data) for _ in range(10)]
labeled_data = []
for aug_data in augmented_data_list:
    input_data = aug_data[:30].reshape(1, 30, 4)
    label = user_label(input_data[0])
    labeled_data.append((input_data[0], label))

# Prepare training data
training_data = np.array([data for data, label in labeled_data])
labels = np.array([label for _, label in labeled_data]).reshape(-1, 1)

# Fine-tune the VAE on user-labeled data
vae.fit(x=(training_data, labels), y=training_data, epochs=20, batch_size=BATCH_SIZE)

# Train the classifier on the initial data
classifier.fit(training_data, labels, epochs=5, batch_size=5, verbose=0)

# Start the iterative training loop
iterative_training_loop_with_classifier_guidance(
    decoder, classifier, vae, iterations=100000
)
