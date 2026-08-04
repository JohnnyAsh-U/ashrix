package policy


type PolicyHandler struct {
	store Repository
	distributor *PolicyDistributor
}

func NewPolicyHandler(store Repository, distributor *PolicyDistributor) *PolicyHandler {
	return &PolicyHandler{store: store, distributor: distributor}
}

// @Summary Create a new policy
// @Description Create a new policy.
// @Tags Policy
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param connector body CreateConnector true "Connector details"
// @Success 201 {object} ConnectorResponse
// @Failure 400 {object} dto.AppError
// @Failure 500 {object} dto.AppError
// @Router /connectors [post]