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
	RequestParameters   map[string]any `json:"requestParameters" yaml:"requestParameters"`
} // @name "x-amazon-apigateway-integration"

var VPCLinkId = ""

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
	aws.ConnectionId = fmt.Sprintf(`!ImportValue VpcLinkId%v`, VPCLinkId)
	// FIX: Parse as raw value. Requires `Port` variable to be defined in
	aws.URI = fmt.Sprintf(`!Sub "https://${InternalDomainName}:${Port}%v%v`, operation.parser.swagger.BasePath, mainRoute.Path)

	aws.ConnectionType = "VPC_LINK"
	aws.ContentHandling = "CONVERT_TO_TEXT"
	aws.PassthroughBehavior = "when_no_match"
	aws.HTTPMethod = mainRoute.HTTPMethod
	aws.Type = "HTTP"

	return aws
}

func requestParams(operation Operation) map[string]any {
	params := map[string]any{}

	for _, param := range operation.Parameters {
		if param.In != "body" && param.Name != "" {
			integrationStr := fmt.Sprintf(`integration.request.%v.%v`, param.In, param.Name)
			methodStr := fmt.Sprintf(`method.request.%v.%v`, param.In, param.Name)
			params[integrationStr] = methodStr
		}
	}

	return params
}

func responseParams(operation Operation, statusCode int) map[string]any {
	params := map[string]any{}

	// Add CORS headers
	params["method.response.header.Access-Control-Allow-Origin"] = "*"

	res := operation.Responses.StatusCodeResponses[statusCode]

	for name := range res.Headers {
		methodStr := fmt.Sprintf(`method.response.header.%v`, name)
		integrationStr := fmt.Sprintf(`integration.response.header.%v`, name)
		params[methodStr] = integrationStr
	}

	return params
}

func parseSimpleStatusCodes(statuses []string, operation Operation) (APIGateway, error) {
	aws := newAWSGatewayIntegration(operation)

	aws.RequestParameters = requestParams(operation)

	for _, codeStr := range statuses {
		statusCode, err := strconv.Atoi(strings.TrimSpace(codeStr))
		if err != nil {
			return aws, fmt.Errorf("can not parse response comment \"%v\"", codeStr)
		}

		codeStr := strings.TrimSpace(codeStr)

		aws.Responses[codeStr] = map[string]any{
			"statusCode":         codeStr,
			"responseParameters": responseParams(operation, statusCode),
		}
	}

	return aws, nil
}

func parseResponseStatusCodes(operation Operation) (APIGateway, error) {
	aws := newAWSGatewayIntegration(operation)

	aws.RequestParameters = requestParams(operation)

	for status := range operation.Responses.StatusCodeResponses {
		statusCode := fmt.Sprintf("%v", status)
		aws.Responses[statusCode] = map[string]any{
			"statusCode":         status,
			"responseParameters": responseParams(operation, status),
		}
	}

	return aws, nil
}
