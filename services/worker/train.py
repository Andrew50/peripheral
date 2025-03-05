import os
import random
import datetime
import numpy as np
import tensorflow as tf
import keras_cv
from tensorflow.keras import layers
from tensorflow.keras.models import Sequential
from tensorflow.keras.layers import (
    Dense,
    LSTM,
    LayerNormalization,
    Bidirectional,
    Conv1D,
    Input,
)
from tensorflow.keras.optimizers import Adam
from tensorflow.keras.callbacks import (
    EarlyStopping,
    ReduceLROnPlateau,
)
from google.protobuf import text_format
from tensorflow_serving.config import model_server_config_pb2
from data import getTensor
from tensorflow.keras.initializers import Constant

# tf.get_logger().setLevel('DEBUG')

tf.config.threading.set_intra_op_parallelism_threads(4)
tf.config.threading.set_inter_op_parallelism_threads(4)
print("CPUs available:", len(tf.config.experimental.list_physical_devices("CPU")))
print("TF_INTER_OP_PARALLELISM_THREADS:", os.environ.get("TF_NUM_INTEROP_THREADS"))
print("TF_INTRA_OP_PARALLELISM_THREADS:", os.environ.get("TF_NUM_INTRAOP_THREADS"))

SEED = 42

np.random.seed(SEED)
tf.random.set_seed(SEED)
random.seed(SEED)
os.environ["PYTHONHASHSEED"] = str(SEED)
os.environ["TF_DETERMINISTIC_OPS"] = "1"


SPLIT_RATIO = 0.75
TRAINING_CLASS_RATIO = 0.02  # .18
VALIDATION_CLASS_RATIO = 0.02
MIN_RANDOM_NOS = 0.65
MAX_EPOCHS = 500
PATIENCE_EPOCHS = 50
LEARNING_RATE_PATIENCE_EPOCHS = 20
LEARNING_RATE_CUT = 0.8
MIN_EPOCHS = 15
BATCH_SIZE = 64
LEARNING_RATE = 5e-4  # inital
CONV_FILTER_UNITS = [9]
CONV_FILTER_KERNAL_SIZES = [5]
BI_LSTM_UNITS = [32]
DROPOUT_PERCENTS = []
BI_LISM_DROPOUTS = [0.75]
DROPOUT_LAYERS = []
NORMALIZATION_TYPE = "rolling-log"  # insance: rolling-log (with tanh conv activation)  #good = min-max, z-score,
NUM_HEADS = 4  # Number of attention heads
D_MODEL = 64  # Dimensionality of the output space of the attention
FF_DIM = 128  # Dimensionality of the feedforward network


def createModel(initial_bias=None):
    model = Sequential()
    model.add(Input(shape=(None, 4)))  # Assuming OHLC data
    model.add(
        Conv1D(
            filters=32,
            kernel_size=5,
            activation="tanh",
            kernel_initializer=tf.keras.initializers.GlorotUniform(seed=SEED),
        )
    )
    model.add(LayerNormalization())
    model.add(
        Bidirectional(
            LSTM(
                units=64,
                return_sequences=False,
                dropout=0.65,
                recurrent_initializer=tf.keras.initializers.GlorotUniform(seed=SEED),
            )
        )
    )
    output_bias = Constant(initial_bias) if initial_bias is not None else None
    model.add(
        Dense(
            1,
            activation="sigmoid",
            bias_initializer=output_bias,
            kernel_initializer=tf.keras.initializers.GlorotUniform(seed=SEED),
        )
    )
    focal_loss = keras_cv.losses.FocalLoss(alpha=0.25, gamma=2, from_logits=False)
    model.compile(
        optimizer=Adam(learning_rate=LEARNING_RATE),
        # loss='binary_crossentropy',
        loss=focal_loss,
        metrics=[tf.keras.metrics.AUC(curve="PR", name="auc_pr")],
    )
    return model


def train_model(conn, setupID):
    with conn.db.cursor() as cursor:
        cursor.execute(
            "SELECT timeframe, bars, modelVersion, untrainedSamples FROM setups WHERE setupId = %s",
            (setupID,),
        )
        traits = cursor.fetchone()
    interval = traits[0]
    if interval != "1d":
        return "invalid tf"
    bars = traits[1]
    modelVersion = traits[2] + 1
    addedSamples = traits[3]

    trainingSample, validationSample = getSample(
        conn,
        setupID,
        interval,
        TRAINING_CLASS_RATIO,
        VALIDATION_CLASS_RATIO,
        SPLIT_RATIO,
    )
    xTrainingData, yTrainingData = getTensor(
        conn, trainingSample, interval, bars, normalize=NORMALIZATION_TYPE
    )
    yTrainingData = np.array([m["label"] for m in yTrainingData])
    xValidationData, yValidationData = getTensor(
        conn, validationSample, interval, bars, normalize=NORMALIZATION_TYPE
    )
    yValidationData = np.array([m["label"] for m in yValidationData])
    yTrainingData, yValidationData = np.array(yTrainingData, dtype=np.int32), np.array(
        yValidationData, dtype=np.int32
    )
    pos = np.sum(yTrainingData)
    neg = len(yTrainingData) - pos
    initial_bias = np.log([pos / neg]) if neg > 0 else 0
    model = createModel(initial_bias)

    validationRatio = np.mean(yValidationData)
    trainingRatio = np.mean(yTrainingData)
    weight_yes = 1.0 / trainingRatio
    weight_no = 1.0 / (1.0 - trainingRatio)
    class_weight = {0: weight_no, 1: weight_yes}
    print(
        f"{len(xValidationData) * validationRatio + len(xTrainingData) * trainingRatio} yes samples"
    )
    print("training class ratio", trainingRatio)
    print("validation class ratio", validationRatio)
    print("training sample size", len(xTrainingData))
    print("class weights", class_weight)
    early_stopping = EarlyStopping(
        # monitor='val_auc_pr',
        monitor="val_auc_pr",
        patience=PATIENCE_EPOCHS,
        restore_best_weights=True,
        start_from_epoch=MIN_EPOCHS,
        # mode='max',
        mode="min",
        verbose=1,
    )
    lr_scheduler = ReduceLROnPlateau(
        # monitor='val_auc_pr',
        monitor="val_loss",
        factor=LEARNING_RATE_CUT,
        patience=int(LEARNING_RATE_PATIENCE_EPOCHS),
        min_lr=1e-6,
        # mode='max',
        mode="min",
        verbose=1,
    )
    history = model.fit(
        xTrainingData,
        yTrainingData.reshape(-1, 1),
        shuffle=False,
        epochs=MAX_EPOCHS,
        batch_size=BATCH_SIZE,
        validation_data=(xValidationData, yValidationData.reshape(-1, 1)),
        callbacks=[early_stopping, lr_scheduler],
        # class_weight=class_weight,
    )
    # history = model.fit(xTrainingData, yTrainingData,epochs=MAX_EPOCHS,batch_size=BATCH_SIZE,validation_data=(xValidationData, yValidationData),callbacks=[early_stopping])
    tf.keras.backend.clear_session()
    score = round(max(history.history["val_auc_pr"]) * 100)
    with conn.db.cursor() as cursor:
        cursor.execute(
            "UPDATE setups SET score = %s, modelVersion = %s WHERE setupId = %s;",
            (score, modelVersion, setupID),
        )
    conn.db.commit()
    if save:
        save(setupID, modelVersion, model)
    size = None
    for val, ident in [[size, "sampleSize"], [score, "score"]]:
        if val is not None:
            with conn.db.cursor() as cursor:
                query = f"UPDATE setups SET {ident} = %s WHERE setupId = %s;"
                cursor.execute(query, (val, setupID))
    conn.db.commit()
    with conn.db.cursor() as cursor:
        cursor.execute(
            """
            UPDATE setups 
            SET untrainedSamples = untrainedSamples - %s 
            WHERE setupId = %s;
        """,
            (addedSamples, setupID),
        )
    conn.db.commit()
    return score


def save(setupID, modelVersion, model):
    model_folder = f"/models/{setupID}/{modelVersion}"
    configPath = "/models/models.config"
    if not os.path.exists(model_folder):
        os.makedirs(model_folder)
    model.export(model_folder)
    config = model_server_config_pb2.ModelServerConfig()
    with open(configPath, "r") as f:
        text_format.Merge(f.read(), config)
    config_exists = False
    for model in config.model_config_list.config:
        if model.name == str(setupID):
            config_exists = True
            break
    if not config_exists:
        new_model_text = f"""
        model_config_list {{
            config {{
                name: "{setupID}"
                base_path: "/models/{setupID}"
                model_platform: "tensorflow"
                model_version_policy {{ all {{}} }}
            }}
        }}
        """
        new_model_config = model_server_config_pb2.ModelServerConfig()
        text_format.Merge(new_model_text, new_model_config)
        config.model_config_list.config.extend(
            new_model_config.model_config_list.config
        )
        with open(configPath, "w") as f:
            f.write(text_format.MessageToString(config))


def getSample(
    data, setupID, interval, TRAINING_CLASS_RATIO, VALIDATION_CLASS_RATIO, SPLIT_RATIO
):
    b = 3  # exclusive interval scaling factor
    # Define the time delta based on the interval unit (days, weeks, months, etc.)
    if "d" in interval:
        timedelta = datetime.timedelta(days=b * int(interval[:-1]))
    elif "w" in interval:
        timedelta = datetime.timedelta(weeks=b * int(interval[:-1]))
    elif "m" in interval:
        timedelta = datetime.timedelta(weeks=b * int(interval[:-1]))
    elif "h" in interval:
        timedelta = datetime.timedelta(hours=b * int(interval[:-1]))
    else:
        timedelta = datetime.timedelta(minutes=b * int(interval))

    with data.db.cursor() as cursor:
        # Select all positive (TRUE label) samples for the given setupId
        yesQuery = """
        SELECT  sec.ticker, s.timestamp, s.label,s.securityId, sec.minDate, sec.maxDate
        FROM samples s
        JOIN securities sec ON s.securityId = sec.securityId
        WHERE s.setupId = %s AND s.label IS TRUE
        AND s.timestamp >= sec.minDate
        AND (sec.maxDate IS NULL OR s.timestamp < sec.maxDate)
        ORDER BY s.timestamp;
        """
        cursor.execute(yesQuery, (setupID,))
        yesInstances = cursor.fetchall()
        numYes = len(yesInstances)

        # Calculate the required number of training and validation instances
        t = TRAINING_CLASS_RATIO
        v = VALIDATION_CLASS_RATIO
        s = SPLIT_RATIO
        z = t * s + v * (1 - s)
        numYesTraining = int(numYes * (t * s / z))
        numYesValidation = int(numYes * (v * (1 - s) / z))
        numNoTraining = int(numYesTraining * (1 / t - 1))
        numNoValidation = int(numYesValidation * (1 / v - 1))
        totalNo = numNoTraining + numNoValidation

        batch_size = 100
        all_noInstances = []
        max_nearby_no = int(totalNo * (1 - MIN_RANDOM_NOS))
        gotten_nearby_nos = 0

        for i in range(0, len(yesInstances), batch_size):
            # Create a batch of 100 positive instances
            batch = yesInstances[i : i + batch_size]
            unionQuery = []

            for ticker, timestamp, label, securityId, minDate, maxDate in batch:
                unionQuery.append(
                    f"""
                (SELECT s.sampleId, sec.ticker, s.timestamp, s.label
                FROM samples s
                JOIN securities sec ON s.securityId = sec.securityId
                WHERE s.securityId = {securityId}
                AND s.setupId = {setupID}
                AND s.label IS FALSE
                AND s.timestamp >= '{max(minDate, timestamp - timedelta)}'
                AND (sec.maxDate IS NULL OR s.timestamp < '{min(maxDate or timestamp + timedelta, timestamp + timedelta)}'))
                """
                )

            # Combine the batch queries with UNION
            noQuery = f"""
            SELECT * FROM (
                {' UNION '.join(unionQuery)}
            ) AS combined_results
            LIMIT {max_nearby_no - gotten_nearby_nos};
            """
            # Execute the batch query
            cursor.execute(noQuery)
            batch_noInstances = cursor.fetchall()
            all_noInstances.extend(batch_noInstances)
            gotten_nearby_nos += len(batch_noInstances)
            if gotten_nearby_nos >= max_nearby_no:
                break

        # Collect sampleIds of the negative samples (noInstances)
        noIDs = [x[0] for x in all_noInstances]
        noInstances = [
            x[1:] for x in all_noInstances
        ]  # Exclude sampleId from noInstances
        yesInstances = [x[:3] for x in yesInstances]

        ''' # Construct the query to find negative (FALSE label) samples around the positive samples
        max_nearby_no = int(totalNo * (1-MIN_RANDOM_NOS))
        unionQuery = []
        for ticker, timestamp,label,securityId, minDate, maxDate in yesInstances:
            unionQuery.append(f"""
            (SELECT s.sampleId,sec.ticker, s.timestamp, s.label
             FROM samples s
             JOIN securities sec ON s.securityId = sec.securityId
             WHERE s.securityId = {securityId}
             AND s.setupId = {setupID}
             AND s.label IS FALSE
             AND s.timestamp >= '{max(minDate, timestamp - timedelta)}'
             AND (sec.maxDate IS NULL OR s.timestamp < '{min(maxDate or timestamp + timedelta, timestamp + timedelta)}'))
            """)
        
        # Combine all queries with UNION
#        noQuery = ' UNION '.join(unionQuery)
        noQuery = f"""
        SELECT * FROM (
            {' UNION '.join(unionQuery)}
        ) AS combined_results
        LIMIT {max_nearby_no};
        """
        cursor.execute(noQuery)
        noInstances = cursor.fetchall()

        # Collect sampleIds of the negative samples (noInstances)
        noIDs = [x[0] for x in noInstances]
        noInstances = [x[1:] for x in noInstances]  # Exclude sampleId from noInstances
        yesInstances = [x[:3] for x in yesInstances]'''

        # Handle case where additional negative samples are needed
        neededNo = totalNo - len(noInstances)
        if neededNo > 0:
            randomNoQuery = f"""
            SELECT sec.ticker, s.timestamp, s.label
            FROM samples s
            JOIN securities sec ON s.securityId = sec.securityId
            WHERE s.setupId = {setupID}
            AND s.label IS FALSE
            AND s.sampleId NOT IN ({','.join(map(str, noIDs))})
            LIMIT {neededNo};
            """
            cursor.execute(randomNoQuery)
            noInstances += cursor.fetchall()

        # Shuffle and split data into training and validation sets
        random.shuffle(yesInstances)
        random.shuffle(noInstances)
        trainingInstances = yesInstances[:numYesTraining] + noInstances[:numNoTraining]
        validationInstances = (
            yesInstances[numYesTraining:] + noInstances[numNoTraining:]
        )

        random.shuffle(trainingInstances)
        random.shuffle(validationInstances)
        trainingDicts = [
            {"ticker": instance[0], "dt": instance[1], "label": instance[2]}
            for instance in trainingInstances
        ]
        validationDicts = [
            {"ticker": instance[0], "dt": instance[1], "label": instance[2]}
            for instance in validationInstances
        ]

        return trainingDicts, validationDicts


def train(conn, setupId):
    tf.keras.backend.clear_session()
    err = None
    results = None
    try:
        results = train_model(conn, setupId)
    except Exception as e:
        err = e
    finally:
        conn.cache.set(f"{setupId}_train_running", "false")
        if err is not None:
            raise err
    return results
