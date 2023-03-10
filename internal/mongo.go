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

const collectionLog = "sys_log"
const collectionChargePoints = "charge_points"
const collectionConnectors = "connectors"

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

func (m *MongoDB) GetConnectors() ([]models.Connector, error) {
	connection, err := m.connect()
	if err != nil {
		return nil, err
	}
	defer m.disconnect(connection)

	var connectors []models.Connector
	collection := connection.Database(m.database).Collection(collectionChargePoints)
	filter := bson.D{}
	//opts := options.Find()
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
	update := bson.D{
		{"$set", bson.D{
			{"charge_point_id", chargePoint.Id},
			{"vendor", chargePoint.Vendor},
			{"model", chargePoint.Model},
			{"serial_number", chargePoint.SerialNumber},
			{"firmware_version", chargePoint.FirmwareVersion},
			{"status", chargePoint.Status},
			{"is_enabled", chargePoint.IsEnabled},
			{"error_code", chargePoint.ErrorCode},
		}},
	}
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
	update := bson.D{
		{"$set", bson.D{
			{"charge_point_id", connector.ChargePointId},
			{"connector_id", connector.Id},
			{"status", connector.Status},
			{"is_enabled", connector.IsEnabled},
		}},
	}
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
	if err != nil {
		return err
	}
	return nil
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
