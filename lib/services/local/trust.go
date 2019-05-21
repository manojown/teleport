package local

import (
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CA is local implementation of Trust service that
// is using local backend
type CA struct {
	backend.Backend
	// getter is used to make batch requests to the backend.
	getter backend.ItemsGetter
}

// NewCAService returns new instance of CAService
func NewCAService(b backend.Backend) *CA {
	getter, _ := b.(backend.ItemsGetter)
	return &CA{
		getter:  getter,
		Backend: b,
	}
}

// DeleteAllCertAuthorities deletes all certificate authorities of a certain type
func (s *CA) DeleteAllCertAuthorities(caType services.CertAuthType) error {
	return s.DeleteBucket([]string{"authorities"}, string(caType))
}

// CreateCertAuthority updates or inserts a new certificate authority
func (s *CA) CreateCertAuthority(ca services.CertAuthority) error {
	if err := ca.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), ca.Expiry())
	err = s.CreateVal([]string{"authorities", string(ca.GetType())}, ca.GetName(), data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (s *CA) UpsertCertAuthority(ca services.CertAuthority) error {
	if err := ca.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), ca.Expiry())
	err = s.UpsertVal([]string{"authorities", string(ca.GetType())}, ca.GetName(), data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteCertAuthority deletes particular certificate authority
func (s *CA) DeleteCertAuthority(id services.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}
	// when removing a services.CertAuthority also remove any deactivated
	// services.CertAuthority as well if they exist.
	err := s.DeleteKey([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	err = s.DeleteKey([]string{"authorities", string(id.Type)}, id.DomainName)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (s *CA) ActivateCertAuthority(id services.CertAuthID) error {
	data, err := s.GetVal([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName)
	if err != nil {
		return trace.BadParameter("can not activate CertAuthority which has not been deactivated: %v: %v", id, err)
	}

	certAuthority, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.UpsertCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.DeleteKey([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeactivateCertAuthority moves a CertAuthority from the normal list to
// the deactivated list.
func (s *CA) DeactivateCertAuthority(id services.CertAuthID) error {
	certAuthority, err := s.GetCertAuthority(id, true)
	if err != nil {
		return trace.NotFound("can not deactivate CertAuthority which does not exist: %v", err)
	}

	err = s.DeleteCertAuthority(id)
	if err != nil {
		return trace.Wrap(err)
	}

	data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := backend.TTL(s.Clock(), certAuthority.Expiry())

	err = s.UpsertVal([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName, data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (s *CA) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool) (services.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	data, err := s.GetVal([]string{"authorities", string(id.Type)}, id.DomainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ca.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	setSigningKeys(ca, loadSigningKeys)
	return ca, nil
}

func setSigningKeys(ca services.CertAuthority, loadSigningKeys bool) {
	if loadSigningKeys {
		return
	}
	ca.SetSigningKeys(nil)
	keyPairs := ca.GetTLSKeyPairs()
	for i := range keyPairs {
		keyPairs[i].Key = nil
	}
	ca.SetTLSKeyPairs(keyPairs)
}

// DELETE IN: 2.6.0
// GetAnyCertAuthority returns activated or deactivated certificate authority
// by given id whether it is activated or not. This method is used in migrations.
func (s *CA) GetAnyCertAuthority(id services.CertAuthID) (services.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	data, err := s.GetVal([]string{"authorities", string(id.Type)}, id.DomainName)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		data, err = s.GetVal([]string{"authorities", "deactivated", string(id.Type)}, id.DomainName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (s *CA) GetCertAuthorities(caType services.CertAuthType, loadSigningKeys bool) ([]services.CertAuthority, error) {
	cas := []services.CertAuthority{}
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	if s.getter != nil {
		return s.batchGetCertAuthorities(caType, loadSigningKeys)
	}
	domains, err := s.GetKeys([]string{"authorities", string(caType)})
	if err != nil {
		if trace.IsNotFound(err) {
			return cas, nil
		}
		return nil, trace.Wrap(err)
	}
	for _, domain := range domains {
		ca, err := s.GetCertAuthority(services.CertAuthID{DomainName: domain, Type: caType}, loadSigningKeys)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cas = append(cas, ca)
	}
	return cas, nil
}

func (s *CA) batchGetCertAuthorities(caType services.CertAuthType, loadSigningKeys bool) ([]services.CertAuthority, error) {
	bucket := []string{"authorities", string(caType)}
	items, err := s.getter.GetItems(bucket)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cas := make([]services.CertAuthority, len(items))
	for i, item := range items {
		ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(item.Value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := ca.Check(); err != nil {
			return nil, trace.Wrap(err)
		}
		setSigningKeys(ca, loadSigningKeys)
		cas[i] = ca
	}
	return cas, nil
}
