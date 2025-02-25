import tensorflow as tf
from tensorflow.keras.models import Sequential, Model
from tensorflow.keras.layers import (
    Input,
    Dense,
    LSTM,
    Bidirectional,
    Conv1D,
    Dropout,
    MultiHeadAttention,
    LayerNormalization,
    GlobalAveragePooling1D,
    Add,
)
from tensorflow.keras.optimizers import Adam

# Global constants for the transformer model
D_MODEL = 128  # Dimension of the model
NUM_HEADS = 8  # Number of attention heads
FF_DIM = 512  # Feed-forward network dimension
LEARNING_RATE = 1e-3  # Learning rate for optimizer
CONV_FILTER_UNITS = [32, 64, 64]  # Units for Conv1D layers
CONV_FILTER_KERNAL_SIZES = [5, 3, 3]  # Kernel sizes for Conv1D layers
BI_LSTM_UNITS = [64, 32, 16]  # Units for Bidirectional LSTM layers
DROPOUT_PERCENTS = [0.4, 0.3]  # Dropout rates


def positional_encoding(seq_len, d_model):
    positions = tf.cast(tf.range(start=0, limit=seq_len, delta=1), dtype=tf.float32)
    angles = positions[:, tf.newaxis] / tf.pow(
        10000,
        (2 * (tf.cast(tf.range(d_model), tf.float32)[tf.newaxis, :] // 2))
        / tf.cast(d_model, tf.float32),
    )
    pos_encoding = tf.where(
        tf.range(d_model)[tf.newaxis, :] % 2 == 0, tf.sin(angles), tf.cos(angles)
    )
    return pos_encoding


def create_transformer_model(input_shape, num_transformer_blocks=4):
    input_layer = Input(
        shape=(None, input_shape[1])
    )  # Input shape (sequence_length, num_features)

    # Project input to the transformer's model dimension (D_MODEL)
    x = Bidirectional(LSTM(64, return_sequences=True))(input_layer)

    # Project input to the transformer's model dimension (D_MODEL)
    # projected_input = layers.Dense(D_MODEL)(input_layer)
    projected_input = Dense(D_MODEL)(x)

    # Add positional encoding
    seq_len = input_shape[0]
    pos_encoding = positional_encoding(seq_len, D_MODEL)
    pos_encoding = tf.expand_dims(pos_encoding, axis=0)  # Add batch dimension
    projected_input += pos_encoding  # Add positional encoding to the projected input

    # Stack transformer blocks
    x = projected_input
    for _ in range(num_transformer_blocks):
        attn_output = MultiHeadAttention(num_heads=NUM_HEADS, key_dim=D_MODEL)(x, x)
        attn_output = Dropout(0.4)(attn_output)
        attn_output = Add()([x, attn_output])  # Residual connection
        attn_output = LayerNormalization(epsilon=1e-6)(attn_output)

        # Feedforward network
        ffn_output = Dense(FF_DIM, activation="relu")(attn_output)
        ffn_output = Dropout(0.4)(ffn_output)
        ffn_output = Dense(D_MODEL)(ffn_output)
        x = Add()([attn_output, ffn_output])  # Residual connection
        x = LayerNormalization(epsilon=1e-6)(x)

    # Global average pooling and output
    pooled_output = GlobalAveragePooling1D()(x)
    output_layer = Dense(1, activation="sigmoid")(pooled_output)

    model = Model(inputs=input_layer, outputs=output_layer)
    model.compile(
        optimizer=Adam(learning_rate=1e-3),
        loss="binary_crossentropy",
        metrics=[tf.keras.metrics.AUC(curve="PR", name="auc_pr")],
    )
    return model


def _createModel():
    model = Sequential()
    model.add(Input(shape=(None, 4)))  # assuming o h l c
    for units, kernal_size in zip(CONV_FILTER_UNITS, CONV_FILTER_KERNAL_SIZES):
        model.add(Conv1D(filters=units, kernel_size=kernal_size, activation="relu"))
    for units, dropout in zip(BI_LSTM_UNITS[:-1], DROPOUT_PERCENTS):
        model.add(Bidirectional(LSTM(units=units, return_sequences=True)))
        model.add(Dropout(dropout))
    model.add(Bidirectional(LSTM(units=BI_LSTM_UNITS[-1], return_sequences=False)))
    model.add(Dense(1, activation="sigmoid"))
    model.compile(
        optimizer=Adam(learning_rate=LEARNING_RATE),
        loss="binary_crossentropy",
        metrics=[tf.keras.metrics.AUC(curve="PR", name="auc_pr")],
    )
    return model


def ___createModel():
    inputs = Input(shape=(None, 4))  # assuming OHLC
    x = Conv1D(filters=64, kernel_size=5, activation="tanh")(inputs)
    x = LayerNormalization()(x)
    attention_output = MultiHeadAttention(num_heads=8, key_dim=128)(
        x, x
    )  # Using `x` as both query and value
    x = LayerNormalization()(attention_output)
    x = Bidirectional(LSTM(units=128, return_sequences=False, dropout=0.65))(x)
    outputs = Dense(1, activation="sigmoid")(x)
    model = Model(inputs=inputs, outputs=outputs)
    model.compile(
        optimizer=tf.keras.optimizers.Adam(learning_rate=LEARNING_RATE),
        loss="binary_crossentropy",
        metrics=[tf.keras.metrics.AUC(curve="PR", name="auc_pr")],
    )
    return model


def weighted_binary_crossentropy(weight_yes, weight_no):
    """
    Weighted binary cross-entropy loss function.

    Parameters:
    weight_yes (float): Weight for the positive class (1).
    weight_no (float): Weight for the negative class (0).

    Returns:
    loss function
    """

    def loss(y_true, y_pred):
        # Clip predictions to prevent log(0)
        y_pred = tf.clip_by_value(
            y_pred, tf.keras.backend.epsilon(), 1 - tf.keras.backend.epsilon()
        )
        # Compute weighted binary cross-entropy
        loss = -(
            weight_yes * y_true * tf.math.log(y_pred)
            + weight_no * (1 - y_true) * tf.math.log(1 - y_pred)
        )
        return tf.reduce_mean(loss)

    return loss


def focal_loss(gamma=2.0, alpha=0.25):
    """
    Focal loss for binary classification.

    Parameters:
    gamma (float): Focusing parameter to reduce the relative loss for well-classified examples.
    alpha (float): Balancing factor to adjust the loss contribution from positive and negative examples.

    Returns:
    loss function
    """

    def focal_loss_fixed(y_true, y_pred):
        # Clip predictions to prevent log(0)
        y_pred = tf.clip_by_value(
            y_pred, tf.keras.backend.epsilon(), 1 - tf.keras.backend.epsilon()
        )
        # Compute the focal loss components
        alpha_factor = y_true * alpha + (1 - y_true) * (1 - alpha)
        focal_weight = y_true * (1 - y_pred) ** gamma + (1 - y_true) * y_pred**gamma
        loss = -alpha_factor * focal_weight * tf.math.log(y_pred)
        return tf.reduce_mean(loss)

    return focal_loss_fixed
