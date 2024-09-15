import tensorflow as tf,  datetime,random , numpy as np, os
from tensorflow.keras.models import Sequential, Model
from tensorflow.keras.layers import Dense, LSTM, Bidirectional, Dropout, Conv1D, Flatten, Lambda, Input
from tensorflow.keras.optimizers import Adam
from tensorflow.keras.layers import Bidirectional
from tensorflow.keras.callbacks import EarlyStopping
#from imblearn.over_sampling import SMOTE
from google.protobuf import text_format
from tensorflow_serving.config import model_server_config_pb2
from data import getTensor
tf.get_logger().setLevel('DEBUG')
tf.config.threading.set_intra_op_parallelism_threads(8)
tf.config.threading.set_inter_op_parallelism_threads(8)

SPLIT_RATIO = .8
TRAINING_CLASS_RATIO = .30
VALIDATION_CLASS_RATIO = .10
MIN_RANDOM_NOS = .5
MAX_EPOCHS = 1000
PATIENCE_EPOCHS = 80
BATCH_SIZE = 64

def createModel():
    model = Sequential()
    model.add(Input(shape=(None, 4))) # assuming o h l c
    conv_filter = 64 #32
    kernal_size = 3
    lstm_list = [64, 32]
    dropout = .1
    model.add(Conv1D(filters=conv_filter, kernel_size=kernal_size, activation='relu'))
    for units in lstm_list[:-1]:
        model.add(Bidirectional(LSTM(units=units, return_sequences=True)))
        model.add(Dropout(dropout))
    model.add(Bidirectional(LSTM(units=lstm_list[-1], return_sequences=False)))
#    model.add(Flatten()) no need becuase return squecnes = false
    model.add(Dense(1, activation='sigmoid'))
    model.compile(optimizer=Adam(learning_rate=1e-3), loss='binary_crossentropy', metrics=[tf.keras.metrics.AUC(curve='PR', name='auc_pr')])
    return model
    


def train_model(conn,setupID):
    with conn.db.cursor() as cursor:
        cursor.execute('SELECT timeframe, bars, modelVersion FROM setups WHERE setupId = %s', (setupID,))
        traits = cursor.fetchone()
    interval = traits[0]
    if interval != "1d":
        return 'invalid tf'
    bars = traits[1]
    modelVersion = traits[2] + 1
    model = createModel()
    trainingSample, validationSample = getSample(conn,setupID,interval,TRAINING_CLASS_RATIO, VALIDATION_CLASS_RATIO, SPLIT_RATIO)
    #print(trainingSample)
    xTrainingData, yTrainingData = getTensor(conn,trainingSample,interval,bars)
    xValidationData, yValidationData = getTensor(conn,validationSample,interval,bars)
    yTrainingData, yValidationData = np.array(yTrainingData,dtype=np.int32),np.array(yValidationData,dtype=np.int32)

    validationRatio = np.mean(yValidationData)
    trainingRatio = np.mean(yTrainingData)
   # TODO all this needs to be sent to frontend instead of just logged
    print(f"{len(xValidationData) * validationRatio + len(xTrainingData) * trainingRatio} yes samples")
    print("training class ratio",trainingRatio)
    print("validation class ratio", validationRatio)
    print("training sample size", len(xTrainingData))
    early_stopping = EarlyStopping(
        monitor='val_auc_pr',
        patience=PATIENCE_EPOCHS,
        restore_best_weights=True,
        mode='max',
        verbose =1
    )
    history = model.fit(xTrainingData, yTrainingData,epochs=MAX_EPOCHS,batch_size=BATCH_SIZE,validation_data=(xValidationData, yValidationData),callbacks=[early_stopping])
    tf.keras.backend.clear_session()
    score = round(history.history['val_auc_pr'][-1] * 100)
    with conn.db.cursor() as cursor:
        cursor.execute("UPDATE setups SET score = %s, modelVersion = %s WHERE setupId = %s;", (score, modelVersion, setupID))
    conn.db.commit()
    if save:
        save(setupID,modelVersion,model)
    size = None
    for val, ident in [[size,'sampleSize'],[score,'score']]:
        if val != None:
            with conn.db.cursor() as cursor:
                query = f"UPDATE setups SET {ident} = %s WHERE setupId = %s;"
                cursor.execute(query, (val, setupID))
    conn.db.commit()
    return score 

def save(setupID,modelVersion,model):
    model_folder = f"models/{setupID}/{modelVersion}"
    configPath = "models/models.config"
    if not os.path.exists(model_folder):
        os.makedirs(model_folder)
    model.export(model_folder)
    config = model_server_config_pb2.ModelServerConfig()
    with open(configPath, 'r') as f:
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
        config.model_config_list.config.extend(new_model_config.model_config_list.config)
        with open(configPath, 'w') as f:
            f.write(text_format.MessageToString(config))

def getSample(data, setupID, interval, TRAINING_CLASS_RATIO, VALIDATION_CLASS_RATIO, SPLIT_RATIO):
    b = 3  # exclusive interval scaling factor
    # Define the time delta based on the interval unit (days, weeks, months, etc.)
    if 'd' in interval:
        timedelta = datetime.timedelta(days=b * int(interval[:-1]))
    elif 'w' in interval:
        timedelta = datetime.timedelta(weeks=b * int(interval[:-1]))
    elif 'm' in interval:
        timedelta = datetime.timedelta(weeks=b * int(interval[:-1]))
    elif 'h' in interval:
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

        # Construct the query to find negative (FALSE label) samples around the positive samples
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
        yesInstances = [x[:3] for x in yesInstances]

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
        validationInstances = yesInstances[numYesTraining:] + noInstances[numNoTraining:]

        random.shuffle(trainingInstances)
        random.shuffle(validationInstances)
        trainingDicts = [{'ticker': instance[0], 'dt': instance[1], 'label': instance[2]} for instance in trainingInstances]
        validationDicts = [{'ticker': instance[0], 'dt': instance[1], 'label': instance[2]} for instance in validationInstances]

        return trainingDicts,validationDicts


def train(data,setupId):
    results = train_model(data,setupId)
    return results

