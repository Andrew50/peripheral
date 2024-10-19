# Define a new model that reuses the pretrained LSTM layers for pattern classification
class PatternClassifier(tf.keras.Model):
    def __init__(self, pretrained_model):
        super(PatternClassifier, self).__init__()
        self.lstm = pretrained_model.lstm  # Use the pretrained LSTM layers
        self.fc1 = tf.keras.layers.Dense(50, activation='relu')  # New dense layer for feature extraction
        self.fc2 = tf.keras.layers.Dense(2, activation='softmax')  # Output layer for binary classification (pattern or not)

    def call(self, inputs):
        x = self.lstm(inputs)  # Use the pretrained LSTM to extract features
        x = self.fc1(x)
        classification = self.fc2(x)
        return classification

# Load the pretrained model and create a new pattern classifier
pattern_classifier = PatternClassifier(pretrain_model)

# Compile the new classifier model
pattern_classifier.compile(optimizer=tf.keras.optimizers.Adam(learning_rate=1e-4),
                           loss='sparse_categorical_crossentropy', metrics=['accuracy'])

# Assuming X_train_patterns is your input time series and y_train_patterns are the pattern labels
pattern_classifier.fit(X_train_patterns, y_train_patterns, epochs=10, batch_size=32)
for layer in pattern_classifier.lstm.layers:
    layer.trainable = False  # Freeze pretrained layers
#or
pattern_classifier.compile(optimizer=tf.keras.optimizers.Adam(learning_rate=1e-5),
                           loss='sparse_categorical_crossentropy', metrics=['accuracy'])

