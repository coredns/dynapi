package dynapi

import (
	"net/http"
	"strings"

	"github.com/miekg/dns"
)

type recordInput struct {
	Zone string `description:"Configured zone." maxLength:"254" path:"zone"`
	Name string `description:"Full owner name." maxLength:"254" path:"name"`

	Type string `description:"A or AAAA." path:"type" pattern:"^(?:[aA]|[aA]{4})$"`
}

func (recordInput *recordInput) resource(origin string) (recordResource, error) {
	zone, valid := canonicalName(recordInput.Zone)
	if !valid || zone != origin {
		return recordResource{}, &apiError{
			status:  http.StatusNotFound,
			code:    codeZoneNotFound,
			message: "zone not found",
		}
	}

	name, valid := canonicalName(recordInput.Name)
	// Both names are literal and canonical, so a suffix with a label boundary
	// is enough. DNS label parsing would allocate slices for each request.
	prefix, inZone := strings.CutSuffix(name, zone)
	if !valid || !inZone || (prefix != "" && !strings.HasSuffix(prefix, ".")) {
		return recordResource{}, &apiError{
			status:  http.StatusUnprocessableEntity,
			code:    codeInvalidName,
			message: "name must be a literal name within the configured zone",
		}
	}

	rrtype := dns.StringToType[strings.ToUpper(recordInput.Type)]
	if rrtype != dns.TypeA && rrtype != dns.TypeAAAA {
		return recordResource{}, &apiError{
			status: http.StatusUnprocessableEntity,
			code:   codeUnsupportedRecordType, message: "type must be A or AAAA",
		}
	}

	return recordResource{name: name, rrtype: rrtype}, nil
}
