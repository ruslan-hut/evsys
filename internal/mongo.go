package internal

import (
	"context"
	"evsys/internal/config"
	"evsys/models"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

const (
	collectionLog           = "sys_log"
	collectionUserTags      = "user_tags"
	collectionChargePoints  = "charge_points"
	collectionConnectors    = "connectors"
	collectionTransactions  = "transactions"
	collectionSubscriptions = "subscriptions"
)

type MongoDB struct {
	ctx           context.Context
	clientOptions *options.ClientOptions
	database      string
}

func NewMongoClient(conf *config.Config) (*MongoDB, error) {
	if !conf.Mongo.Enabled {
		return nil, nil
	}
	connectionUri := fmt.Sprintf("mongodb://%s:%s", conf.Mongo.Host, conf.Mongo.Port)
	clientOptions := options.Client().ApplyURI(connectionUri)
	if conf.Mongo.User != "" {
		clientOptions.SetAuth(options.Credential{
			Username:   conf.Mongo.User,
			Password:   conf.Mongo.Password,
			AuthSource: conf.Mongo.Database,
		})
	}
	client := &MongoDB{
		ctx:           context.Background(),
		clientOptions: clientOptions,
		database:      conf.Mongo.Database,
	}
	return client, nil
}

func (m *MongoDB) connect() (*mongo.Client, error) {
	connection, err := mongo.Connect(m.ctx, m.clientOptions)
	if err != nil {
		return nil, err
	}
	return connection, nil
}

func (m *MongoDB) disconnect(connection *mongo.Client) {
	err := connection.Disconnect(m.ctx)
	if err != nil {
		log.Println("mongodb disconnect error;", err)
	}
}

func (m *MongoDB) Write(table string, data Data) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(table)
	_, err = collection.InsertOne(m.ctx, data)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) WriteLogMessage(data Data) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)
	collection := connection.Database(m.database).Collection(collectionLog)
	_, err = collection.InsertOne(m.ctx, data)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) ReadLog() (interface{}, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	var logMessages []FeatureLogMessage
	collection := connection.Database(m.database).Collection(collectionLog)
	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{"time", -1}}).SetLimit(1000)
	cursor, err := collection.Find(m.ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(m.ctx, &logMessages); err != nil {
		return nil, err
	}
	return logMessages, nil
}

func (m *MongoDB) GetChargePoints() ([]models.ChargePoint, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	var chargePoints []models.ChargePoint
	collection := connection.Database(m.database).Collection(collectionChargePoints)
	filter := bson.D{}
	//opts := options.Find()
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(m.ctx, &chargePoints); err != nil {
		return nil, err
	}
	return chargePoints, nil
}

func (m *MongoDB) GetConnectors() ([]*models.Connector, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	var connectors []*models.Connector
	collection := connection.Database(m.database).Collection(collectionConnectors)
	filter := bson.D{}
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(m.ctx, &connectors); err != nil {
		return nil, err
	}
	return connectors, nil
}

func (m *MongoDB) UpdateChargePoint(chargePoint *models.ChargePoint) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"charge_point_id", chargePoint.Id}}
	update := bson.M{"$set": chargePoint}
	collection := connection.Database(m.database).Collection(collectionChargePoints)
	_, err = collection.UpdateOne(m.ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) AddChargePoint(chargePoint *models.ChargePoint) error {
	existedChargePoint, _ := m.GetChargePoint(chargePoint.Id)
	if existedChargePoint != nil {
		return fmt.Errorf("charge point with id %s already exists", chargePoint.Id)
	}

	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionChargePoints)
	_, err = collection.InsertOne(m.ctx, chargePoint)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) GetChargePoint(id string) (*models.ChargePoint, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"charge_point_id", id}}
	collection := connection.Database(m.database).Collection(collectionChargePoints)
	var chargePoint models.ChargePoint
	err = collection.FindOne(m.ctx, filter).Decode(&chargePoint)
	if err != nil {
		return nil, err
	}
	return &chargePoint, nil
}

func (m *MongoDB) UpdateConnector(connector *models.Connector) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"connector_id", connector.Id}, {"charge_point_id", connector.ChargePointId}}
	update := bson.M{"$set": connector}
	collection := connection.Database(m.database).Collection(collectionConnectors)
	_, err = collection.UpdateOne(m.ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

func (m *MongoDB) AddConnector(connector *models.Connector) error {
	existedConnector, _ := m.GetConnector(connector.Id, connector.ChargePointId)
	if existedConnector != nil {
		return fmt.Errorf("connector with id %v@%s already exists", existedConnector.Id, existedConnector.ChargePointId)
	}
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionConnectors)
	_, err = collection.InsertOne(m.ctx, connector)
	return err
}

func (m *MongoDB) GetConnector(id int, chargePointId string) (*models.Connector, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"connector_id", id}, {"charge_point_id", chargePointId}}
	collection := connection.Database(m.database).Collection(collectionConnectors)
	var connector models.Connector
	err = collection.FindOne(m.ctx, filter).Decode(&connector)
	if err != nil {
		return nil, err
	}
	return &connector, nil
}

func (m *MongoDB) GetUserTag(id string) (*models.UserTag, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"id_tag", id}}
	collection := connection.Database(m.database).Collection(collectionUserTags)
	var userTag models.UserTag
	err = collection.FindOne(m.ctx, filter).Decode(&userTag)
	if err != nil {
		return nil, err
	}
	return &userTag, nil
}

func (m *MongoDB) AddUserTag(userTag *models.UserTag) error {
	existedTag, _ := m.GetUserTag(userTag.IdTag)
	if existedTag != nil {
		return fmt.Errorf("ID tag %s is already registered", existedTag.IdTag)
	}
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionUserTags)
	_, err = collection.InsertOne(m.ctx, userTag)
	return err
}

func (m *MongoDB) GetLastTransaction() (*models.Transaction, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	filter := bson.D{}
	collection := connection.Database(m.database).Collection(collectionTransactions)
	opts := options.FindOne().SetSort(bson.D{{"transaction_id", -1}})
	var transaction models.Transaction
	err = collection.FindOne(m.ctx, filter, opts).Decode(&transaction)
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (m *MongoDB) GetTransaction(id int) (*models.Transaction, error) {
	var transaction models.Transaction
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"transaction_id", id}}
	collection := connection.Database(m.database).Collection(collectionTransactions)
	err = collection.FindOne(m.ctx, filter).Decode(&transaction)
	if err != nil {
		return nil, err
	}
	return &transaction, nil
}

func (m *MongoDB) AddTransaction(transaction *models.Transaction) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionTransactions)
	_, err = collection.InsertOne(m.ctx, transaction)
	return err
}

func (m *MongoDB) UpdateTransaction(transaction *models.Transaction) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"transaction_id", transaction.Id}}
	update := bson.M{"$set": transaction}
	collection := connection.Database(m.database).Collection(collectionTransactions)
	_, err = collection.UpdateOne(m.ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

// GetSubscriptions returns all subscriptions
func (m *MongoDB) GetSubscriptions() ([]models.UserSubscription, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	filter := bson.D{}
	collection := connection.Database(m.database).Collection(collectionSubscriptions)
	cursor, err := collection.Find(m.ctx, filter)
	if err != nil {
		return nil, err
	}
	var subscriptions []models.UserSubscription
	if err = cursor.All(m.ctx, &subscriptions); err != nil {
		return nil, err
	}
	return subscriptions, nil
}

// GetSubscription returns a subscription by user id
func (m *MongoDB) GetSubscription(id int) (*models.UserSubscription, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"user_id", id}}
	collection := connection.Database(m.database).Collection(collectionSubscriptions)
	var subscription models.UserSubscription
	err = collection.FindOne(m.ctx, filter).Decode(&subscription)
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

// AddSubscription adds a new subscription
func (m *MongoDB) AddSubscription(subscription *models.UserSubscription) error {
	existedSubscription, _ := m.GetSubscription(subscription.UserID)
	if existedSubscription != nil {
		return fmt.Errorf("user is already subscribed")
	}
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	collection := connection.Database(m.database).Collection(collectionSubscriptions)
	_, err = collection.InsertOne(m.ctx, subscription)
	return err
}

// DeleteSubscription deletes a subscription
func (m *MongoDB) DeleteSubscription(subscription *models.UserSubscription) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"user_id", subscription.UserID}}
	collection := connection.Database(m.database).Collection(collectionSubscriptions)
	_, err = collection.DeleteOne(m.ctx, filter)
	return err
}

// UpdateSubscription updates a subscription
func (m *MongoDB) UpdateSubscription(subscription *models.UserSubscription) error {
	connection, err := m.connect()
	if err != nil {
		return err
	}
	defer m.disconnect(connection)

	filter := bson.D{{"user_id", subscription.UserID}}
	update := bson.M{"$set": subscription}
	collection := connection.Database(m.database).Collection(collectionSubscriptions)
	_, err = collection.UpdateOne(m.ctx, filter, update)
	if err != nil {
		return err
	}
	return nil
}

type pipeResult struct {
	ChargePointID string `bson:"_id"`
	Info          string `bson:"info"`
}

// GetLastStatus returns the last status
func (m *MongoDB) GetLastStatus() ([]models.ChargePointStatus, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	var status []models.ChargePointStatus
	collection := connection.Database(m.database).Collection(collectionLog)

	pipeline := bson.A{
		bson.D{{"$match", bson.D{{"feature", "Heartbeat"}}}},
		bson.D{{"$sort", bson.D{{"time", -1}}}},
		bson.D{
			{"$group",
				bson.D{
					{"_id", "$charge_point_id"},
					{"info", bson.D{{"$first", "$time"}}},
				},
			},
		},
		bson.D{{"$sort", bson.D{{"_id", 1}}}},
	}
	var pipeResult []pipeResult
	cursor, err := collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate Heartbeat: %v", err)
	}
	if err = cursor.All(m.ctx, &pipeResult); err != nil {
		return nil, fmt.Errorf("decode Heartbeat: %v", err)
	}

	for _, heartbeat := range pipeResult {
		status = append(status, models.ChargePointStatus{
			ChargePointID: heartbeat.ChargePointID,
			Time:          heartbeat.Info,
		})
	}

	pipeline = bson.A{
		bson.D{{"$match", bson.D{{"feature", "StatusNotification"}}}},
		bson.D{{"$sort", bson.D{{"time", -1}}}},
		bson.D{
			{"$group",
				bson.D{
					{"_id", "$charge_point_id"},
					{"info", bson.D{{"$first", "$text"}}},
				},
			},
		},
	}
	cursor, err = collection.Aggregate(m.ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate StatusNotification: %v", err)
	}
	if err = cursor.All(m.ctx, &pipeResult); err != nil {
		return nil, fmt.Errorf("decode StatusNotification: %v", err)
	}

	for _, statusInfo := range pipeResult {
		for i, s := range status {
			if s.ChargePointID == statusInfo.ChargePointID {
				status[i].Status = statusInfo.Info
			}
		}
	}

	return status, nil
}
