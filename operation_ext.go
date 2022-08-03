package swag

import (
	"fmt"
	"go/ast"
	"regexp"
	"strconv"
	"strings"
)

type APIGateway struct {
	ConnectionType      string         `json:"connectionType" yaml:"connectionType" default:"VPC_LINK"`
	ConnectionId        any            `json:"connectionId" yaml:"connectionId"`
	ContentHandling     string         `json:"contentHandling" yaml:"contentHandling"`
	HTTPMethod          string         `json:"httpMethod" yaml:"httpMethod"`
	PassthroughBehavior string         `json:"passthroughBehavior" yaml:"passthroughBehavior" default:"when_no_match"`
	Type                string         `json:"type" yaml:"type"`
	URI                 any            `json:"uri" yaml:"uri"`
	Responses           map[string]any `json:"responses" yaml:"responses"`
} // @name "x-amazon-apigateway-integration"

var awsGatewayPattern = regexp.MustCompile(`([\w,]+)\s*`)

// @aws.api.gateway []int
// lineRemainder:		200...
func (operation *Operation) ParseAWSGatewayResponseComment(lineRemainder string, astFile *ast.File) error {
	// Find definitions in comment string
	matches := awsGatewayPattern.FindStringSubmatch(lineRemainder)

	aws := APIGateway{Responses: map[string]any{}}

	// Parse each status code
	statuses := strings.Split(lineRemainder, ",")

	// FIX: `awsGatewayPattern` to simplify this condition
	if len(matches)+len(statuses) >= 2 {
		aws, _ = parseSimpleStatusCodes(statuses, *operation)
	} else {
		aws, _ = parseResponseStatusCodes(*operation)
	}

	// Add tags to response schema
	operation.VendorExtensible.AddExtension("x-amazon-apigateway-integration", &aws)
	return nil
}

func newAWSGatewayIntegration(operation Operation) APIGateway {
	aws := APIGateway{Responses: map[string]any{}}

	mainRoute := operation.RouterProperties[0]

	// FIX: Interpolate AccountType or AccountId
	// FIX: Parse as raw value
	aws.ConnectionId = `!ImportValue VpcLinkId{AccountType}`
	// FIX: Parse as raw value. Requires `Port` variable to be defined in
	aws.URI = fmt.Sprintf(`!Sub "https://${InternalDomainName}:{Port}%v`, mainRoute.Path)

	aws.ConnectionType = "VPC_LINK"
	aws.ContentHandling = "CONVERT_TO_TEXT"
	aws.PassthroughBehavior = "when_no_match"
	aws.HTTPMethod = mainRoute.HTTPMethod
	aws.Type = "HTTP"

	return aws
}

func capturedParams(operation Operation) map[string]any {
	responseParams := map[string]any{}

	for _, param := range operation.Parameters {
		methodStr := fmt.Sprintf(`method.response.%v.%v`, param.In, param.Name)
		integrationStr := fmt.Sprintf(`integration.response.%v.%v`, param.In, param.Name)
		responseParams[methodStr] = integrationStr
	}

	return responseParams
}

func parseSimpleStatusCodes(statuses []string, operation Operation) (APIGateway, error) {
	aws := newAWSGatewayIntegration(operation)

	for _, codeStr := range statuses {
		_, err := strconv.Atoi(strings.TrimSpace(codeStr))
		if err != nil {
			return aws, fmt.Errorf("can not parse response comment \"%v\"", codeStr)
		}

		codeStr := strings.TrimSpace(codeStr)

		aws.Responses[codeStr] = map[string]any{
			"statusCode":         codeStr,
			"responseParameters": capturedParams(operation),
		}
	}

	return aws, nil
}

func parseResponseStatusCodes(operation Operation) (APIGateway, error) {
	aws := newAWSGatewayIntegration(operation)

	for status := range operation.Responses.StatusCodeResponses {
		aws.Responses[fmt.Sprintf("%v", status)] = map[string]any{
			"statusCode":         status,
			"responseParameters": capturedParams(operation),
		}
	}

	return aws, nil
}
