package types

const SubProtocol16 = "ocpp1.6"

type AuthorizationStatus string

const (
	AuthorizationStatusAccepted     AuthorizationStatus = "Accepted"
	AuthorizationStatusBlocked      AuthorizationStatus = "Blocked"
	AuthorizationStatusExpired      AuthorizationStatus = "Expired"
	AuthorizationStatusInvalid      AuthorizationStatus = "Invalid"
	AuthorizationStatusConcurrentTx AuthorizationStatus = "ConcurrentTx"
)

type IdTagInfo struct {
	ExpiryDate  *DateTime           `json:"expiryDate,omitempty" validate:"omitempty"`
	ParentIdTag string              `json:"parentIdTag,omitempty" validate:"omitempty,max=20"`
	Status      AuthorizationStatus `json:"status" validate:"required,authorizationStatus"`
}

func NewIdTagInfo(status AuthorizationStatus) *IdTagInfo {
	return &IdTagInfo{Status: status}
}

type ReadingContext string
type ValueFormat string
type Measurand string
type Phase string
type Location string
type UnitOfMeasure string

const (
	ReadingContextInterruptionBegin       ReadingContext = "Interruption.Begin"
	ReadingContextInterruptionEnd         ReadingContext = "Interruption.End"
	ReadingContextOther                   ReadingContext = "Other"
	ReadingContextSampleClock             ReadingContext = "Sample.Clock"
	ReadingContextSamplePeriodic          ReadingContext = "Sample.Periodic"
	ReadingContextTransactionBegin        ReadingContext = "Transaction.Begin"
	ReadingContextTransactionEnd          ReadingContext = "Transaction.End"
	ReadingContextTrigger                 ReadingContext = "Trigger"
	ValueFormatRaw                        ValueFormat    = "Raw"
	ValueFormatSignedData                 ValueFormat    = "SignedData"
	MeasurandCurrentExport                Measurand      = "Current.Export"
	MeasurandCurrentImport                Measurand      = "Current.Import"
	MeasurandCurrentOffered               Measurand      = "Current.Offered"
	MeasurandEnergyActiveExportRegister   Measurand      = "Energy.Active.Export.Register"
	MeasurandEnergyActiveImportRegister   Measurand      = "Energy.Active.Import.Register"
	MeasurandEnergyReactiveExportRegister Measurand      = "Energy.Reactive.Export.Register"
	MeasurandEnergyReactiveImportRegister Measurand      = "Energy.Reactive.Import.Register"
	MeasurandEnergyActiveExportInterval   Measurand      = "Energy.Active.Export.Interval"
	MeasurandEnergyActiveImportInterval   Measurand      = "Energy.Active.Import.Interval"
	MeasurandEnergyReactiveExportInterval Measurand      = "Energy.Reactive.Export.Interval"
	MeasurandEnergyReactiveImportInterval Measurand      = "Energy.Reactive.Import.Interval"
	MeasurandFrequency                    Measurand      = "Frequency"
	MeasurandPowerActiveExport            Measurand      = "Power.Active.Export"
	MeasurandPowerActiveImport            Measurand      = "Power.Active.Import"
	MeasurandPowerFactor                  Measurand      = "Power.Factor"
	MeasurandPowerOffered                 Measurand      = "Power.Offered"
	MeasurandPowerReactiveExport          Measurand      = "Power.Reactive.Export"
	MeasurandPowerReactiveImport          Measurand      = "Power.Reactive.Import"
	MeasurandRPM                          Measurand      = "RPM"
	MeasueandSoC                          Measurand      = "SoC"
	MeasurandTemperature                  Measurand      = "Temperature"
	MeasurandVoltage                      Measurand      = "Voltage"
	PhaseL1                               Phase          = "L1"
	PhaseL2                               Phase          = "L2"
	PhaseL3                               Phase          = "L3"
	PhaseN                                Phase          = "N"
	PhaseL1N                              Phase          = "L1-N"
	PhaseL2N                              Phase          = "L2-N"
	PhaseL3N                              Phase          = "L3-N"
	PhaseL1L2                             Phase          = "L1-L2"
	PhaseL2L3                             Phase          = "L2-L3"
	PhaseL3L1                             Phase          = "L3-L1"
	LocationBody                          Location       = "Body"
	LocationCable                         Location       = "Cable"
	LocationEV                            Location       = "EV"
	LocationInlet                         Location       = "Inlet"
	LocationOutlet                        Location       = "Outlet"
	UnitOfMeasureWh                       UnitOfMeasure  = "Wh"
	UnitOfMeasureKWh                      UnitOfMeasure  = "kWh"
	UnitOfMeasureVarh                     UnitOfMeasure  = "varh"
	UnitOfMeasureKvarh                    UnitOfMeasure  = "kvarh"
	UnitOfMeasureW                        UnitOfMeasure  = "W"
	UnitOfMeasureKW                       UnitOfMeasure  = "kW"
	UnitOfMeasureVA                       UnitOfMeasure  = "VA"
	UnitOfMeasureKVA                      UnitOfMeasure  = "kVA"
	UnitOfMeasureVar                      UnitOfMeasure  = "var"
	UnitOfMeasureKvar                     UnitOfMeasure  = "kvar"
	UnitOfMeasureA                        UnitOfMeasure  = "A"
	UnitOfMeasureV                        UnitOfMeasure  = "V"
	UnitOfMeasureCelsius                  UnitOfMeasure  = "Celsius"
	UnitOfMeasureFahrenheit               UnitOfMeasure  = "Fahrenheit"
	UnitOfMeasureK                        UnitOfMeasure  = "K"
	UnitOfMeasurePercent                  UnitOfMeasure  = "Percent"
)

type SampledValue struct {
	Value     string         `json:"value" validate:"required"`
	Context   ReadingContext `json:"context,omitempty" validate:"omitempty,readingContext"`
	Format    ValueFormat    `json:"format,omitempty" validate:"omitempty,valueFormat"`
	Measurand Measurand      `json:"measurand,omitempty" validate:"omitempty,measurand"`
	Phase     Phase          `json:"phase,omitempty" validate:"omitempty,phase"`
	Location  Location       `json:"location,omitempty" validate:"omitempty,location"`
	Unit      UnitOfMeasure  `json:"unit,omitempty" validate:"omitempty,unitOfMeasure"`
}

type MeterValue struct {
	Timestamp    *DateTime      `json:"timestamp" validate:"required"`
	SampledValue []SampledValue `json:"sampledValue" validate:"required,min=1,dive"`
}
