import numpy as np
import tensorflow as tf
import matplotlib.pyplot as plt
import mplfinance as mpf
import pandas as pd
from tensorflow.keras import layers, models, optimizers
from tensorflow.keras.models import Sequential, Model
from tensorflow.keras.layers import Dense, LSTM, Bidirectional, Dropout, Conv1D, Flatten, Lambda, Input
from tensorflow.keras.optimizers import Adam
from tensorflow.keras.layers import Bidirectional
from tensorflow.keras.callbacks import EarlyStopping

SPLIT_RATIO = .8
TRAINING_CLASS_RATIO = .18
VALIDATION_CLASS_RATIO = .065
MIN_RANDOM_NOS = .7
MAX_EPOCHS = 2000
PATIENCE_EPOCHS = 150
MIN_EPOCHS = 50
BATCH_SIZE = 64
CONV_FILTER_UNITS= [64]
CONV_FILTER_KERNAL_SIZES = [5]
BI_LSTM_UNITS = [64,16]
DROPOUT_PERCENTS = [.5]

def createModel():
    model = Sequential()
    model.add(Input(shape=(None, 4))) # assuming o h l c
    for units,kernal_size in zip(CONV_FILTER_UNITS,CONV_FILTER_KERNAL_SIZES):
        model.add(Conv1D(filters=units, kernel_size=kernal_size, activation='relu'))
    for units,dropout in zip(BI_LSTM_UNITS[:-1],DROPOUT_PERCENTS):
        model.add(Bidirectional(LSTM(units=units, return_sequences=True)))
        model.add(Dropout(dropout))
    model.add(Bidirectional(LSTM(units=BI_LSTM_UNITS[-1], return_sequences=False)))
    model.add(Dense(1, activation='sigmoid'))
    model.compile(optimizer=Adam(learning_rate=1e-3), loss='binary_crossentropy', 
                  metrics=[tf.keras.metrics.AUC(curve='PR', name='auc_pr')])
    return model

def reconstruction_loss(input_data, generated_data):
    # Compute L1 loss as the reconstruction loss
    return tf.reduce_mean(tf.abs(input_data - generated_data))

def train_gan_with_reconstruction_loss(gan, generator, discriminator, data, epochs=100, initial_lambda_reconstruction=0.5):
    batch_size = data.shape[0]
    real_labels = np.ones((batch_size, 1))
    fake_labels = np.zeros((batch_size, 1))
    lambda_reconstruction = initial_lambda_reconstruction
    
    # Create an optimizer for the generator
    generator_optimizer = Adam(learning_rate=1e-4)

    for epoch in range(epochs):
        # Train the discriminator on real data
        d_loss_real = discriminator.train_on_batch(data, real_labels)

        # Generate fake data using the generator
        noise = np.random.normal(0, 1, (batch_size, 100))
        generated_data = generator.predict(noise)

        # Train the discriminator on fake data
        d_loss_fake = discriminator.train_on_batch(generated_data, fake_labels)
        
        # Extract the loss value
        d_loss_fake_value = d_loss_fake[0]

        # Calculate reconstruction loss
        recon_loss = reconstruction_loss(data, generated_data)

        # Calculate GAN loss
        g_loss = gan.train_on_batch(noise, real_labels)

        # Update the generator's weights based on total loss
        with tf.GradientTape() as tape:
            generated_data = generator(noise)
            g_loss = gan.train_on_batch(noise, real_labels)
            recon_loss = reconstruction_loss(data, generated_data)
            total_loss = g_loss + lambda_reconstruction * recon_loss

        gradients = tape.gradient(total_loss, generator.trainable_variables)
        generator_optimizer.apply_gradients(zip(gradients, generator.trainable_variables))

        # Optionally adjust lambda_reconstruction based on performance
        if epoch % 10 == 0:
            print(f"Epoch: {epoch}, Discriminator Loss: {d_loss_real[0]}, Generator Loss: {g_loss}, Reconstruction Loss: {recon_loss}")

            # Reduce the influence of reconstruction loss over time if the GAN is performing well
            if d_loss_fake_value < 0.5:
                lambda_reconstruction = max(0.1, lambda_reconstruction * 0.9)  # Decrease, but keep it above 0.1



def user_label(data):
    # Assuming `data` is in the shape (sequence_length, 4)
    df = pd.DataFrame(data, columns=['Open', 'High', 'Low', 'Close'])
    df.index = pd.date_range(start='2023-01-01', periods=len(df), freq='min')  # Create a dummy time index
    
    fig, axlist = mpf.plot(
        df, 
        type='candle', 
        returnfig=True
    )
    
    # Show the plot
    plt.show(block=False)  # Do not block the script execution
    plt.pause(0.1)  # Allow time for the plot to render

    # Capture user input
    while True:
        label = input("Enter label (1 for pattern, 0 for not a pattern): ")
        try:
            intLabel = int(label)
            break
        except:
            
            pass
    plt.close(fig)
    
    return intLabel
def select_uncertain_charts(classifier, generated_charts, max_samples=5):
    # Get predictions for each generated chart
    predictions = classifier.predict(generated_charts)
    
    # Calculate the uncertainty as the absolute distance from 0.5
    uncertainty_scores = np.abs(predictions - 0.5).flatten()
    
    # Get indices of charts with the smallest uncertainty scores
    uncertain_indexes = np.argsort(uncertainty_scores)[:max_samples]
    
    # Select the most uncertain charts to present to the user
    return [generated_charts[i] for i in uncertain_indexes]

def iterative_training_loop_with_classifier_guidance(data,generator, classifier, gan, discriminator, iterations=100):
    labeled_data = []  # To store generated charts and user labels
    for i in range(iterations):
        #noise = np.random.normal(0, 1, (10, 100))  # Generate 10 charts at a time
        #noise = np.random.normal(0, 0.01, (1, 30, 4))  # Slight variations
        noise = np.random.normal(0, 1, (10, 100))
        #generated_charts = generator.predict(noise)  # Add noise to the initial input
        generated_charts = generator.predict(noise)
        uncertain_charts = select_uncertain_charts(classifier, generated_charts)
        for chart in uncertain_charts:
            user_labeling = user_label(chart)  # Plot the chart and get the user's input
            labeled_data.append((chart, user_labeling))
        if len(labeled_data) >= 10:  # Retrain in batches
            charts, labels = zip(*labeled_data)
            charts = np.array(charts)#.reshape(-1, 28, 28, 1)  # Reshape to fit model input
            labels = np.array(labels)
            classifier.fit(charts, labels, epochs=1, batch_size=5, verbose=0)
            positive_examples = [chart for chart, label in labeled_data if label == 1]
            if positive_examples:
                positive_examples = np.array(positive_examples).reshape(-1, 28, 28, 1)
                #train_gan_on_initial_data(gan, generator, discriminator, positive_examples, epochs=10)
                train_gan_with_reconstruction_loss(gan, generator, discriminator, positive_examples, epochs=10)
            labeled_data.clear()
        print(f"Iteration {i + 1}: {len(uncertain_charts)} charts labeled and used for training")


# Create the GAN's Generator model
def build_generator():
    model = models.Sequential()
    model.add(layers.Dense(128, input_dim=100))
    model.add(layers.LeakyReLU(0.2))
    model.add(layers.Dense(256))
    model.add(layers.LeakyReLU(0.2))
    model.add(layers.Dense(30*4, activation='tanh'))  # Assuming output shape for synthetic charts
    model.add(layers.Reshape((30,4)))
    return model

# Modify the GAN's Discriminator model to accept sequences of shape (sequence_length, 4)
def build_discriminator():
    model = models.Sequential()
    model.add(layers.Conv1D(64, kernel_size=3, activation='relu', input_shape=(30, 4)))  # Assuming sequence length of 30
    model.add(layers.Flatten())
    model.add(layers.Dense(256))
    model.add(layers.LeakyReLU(0.2))
    model.add(layers.Dense(1, activation='sigmoid'))
    model.compile(optimizer='adam', loss='binary_crossentropy', metrics=['accuracy'])
    return model

# Create the GAN model
def build_gan(generator, discriminator):
    discriminator.trainable = False  # Freeze discriminator's weights during generator training
    gan_input = layers.Input(shape=(100,))
    generated_image = generator(gan_input)
    gan_output = discriminator(generated_image)
    gan = models.Model(gan_input, gan_output)
    gan.compile(optimizer='adam', loss='binary_crossentropy')
    return gan

# Instantiate the models
generator = build_generator()
discriminator = build_discriminator()
gan = build_gan(generator, discriminator)
classifier = createModel()
data = np.array([
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
])
def normalize_data(data):
    # Min-Max normalization to the range [-1, 1] using global min and max
    global_min = np.min(data)
    global_max = np.max(data)
    normalized_data = 2 * (data - global_min) / (global_max - global_min) - 1
    return normalized_data

# In your main code, normalize the initial training data
initial_data = data[:, 1:][::-1]  # Extract OHLC columns
normalized_initial_data = normalize_data(initial_data)
user_label(normalized_initial_data)
initial_data = normalized_initial_data[:30].reshape(1, 30, 4)
train_gan_with_reconstruction_loss(gan, generator, discriminator, initial_data, epochs=10)
iterative_training_loop_with_classifier_guidance(data,generator, classifier, gan, discriminator, iterations=100000)



