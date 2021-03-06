/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package annotations

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/ingress-gce/pkg/flags"
)

func TestNEGAnnotation(t *testing.T) {
	for _, tc := range []struct {
		desc                string
		svc                 *v1.Service
		expectNegAnnotation *NegAnnotation
		expectError         error
		expectFound         bool
		negEnabled          bool
		ingress             bool
		exposed             bool
	}{
		{
			desc:        "NEG annotation not specified",
			svc:         &v1.Service{},
			expectFound: false,
			expectError: nil,
		},
		{
			desc: "NEG annotation is malformed",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NEGAnnotationKey: `foo`,
					},
				},
			},
			expectFound: true,
			expectError: ErrNEGAnnotationInvalid,
		},
		{
			desc: "NEG annotation is malformed 2",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NEGAnnotationKey: `{"exposed_ports":{80:{}}}`,
					},
				},
			},
			expectFound: true,
			expectError: ErrNEGAnnotationInvalid,
		},
		{
			desc: "NEG enabled for ingress",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NEGAnnotationKey: `{"ingress":true}`,
					},
				},
			},
			expectFound: true,
			expectNegAnnotation: &NegAnnotation{
				Ingress: true,
			},
			negEnabled: true,
			ingress:    true,
			exposed:    false,
		},
		{
			desc: "NEG exposed",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NEGAnnotationKey: `{"exposed_ports":{"80":{}}}`,
					},
				},
			},
			expectFound: true,
			expectNegAnnotation: &NegAnnotation{
				ExposedPorts: map[int32]NegAttributes{int32(80): {}},
			},
			negEnabled: true,
			ingress:    false,
			exposed:    true,
		},
		{
			desc: "Multiple ports exposed",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NEGAnnotationKey: `{"exposed_ports":{"80":{}, "443":{}}}`,
					},
				},
			},
			expectFound: true,
			expectNegAnnotation: &NegAnnotation{
				ExposedPorts: map[int32]NegAttributes{int32(80): {}, int32(443): {}},
			},
			negEnabled: true,
			ingress:    false,
			exposed:    true,
		},
		{
			desc: "Service is enabled for both ingress and exposedNEG",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NEGAnnotationKey: `{"ingress":true,"exposed_ports":{"80":{}, "443":{}}}`,
					},
				},
			},
			expectFound: true,
			expectNegAnnotation: &NegAnnotation{
				Ingress:      true,
				ExposedPorts: map[int32]NegAttributes{int32(80): {}, int32(443): {}},
			},
			negEnabled: true,
			ingress:    true,
			exposed:    true,
		},
	} {
		found, negAnnotation, err := FromService(tc.svc).NEGAnnotation()
		if fmt.Sprintf("%q", err) != fmt.Sprintf("%q", tc.expectError) {
			t.Errorf("Test case %q: Expect error to be %q, but got: %q", tc.desc, tc.expectError, err)
		}

		if found != tc.expectFound {
			t.Errorf("Test case %q: Expect found to be %v, be got %v", tc.desc, tc.expectFound, found)
		}

		if tc.expectError != nil || !tc.expectFound {
			continue
		}

		if !reflect.DeepEqual(*tc.expectNegAnnotation, *negAnnotation) {
			t.Errorf("Test case %q: Expect NegAnnotation to be %v, be got %v", tc.desc, *tc.expectNegAnnotation, *negAnnotation)
		}

		if neg := negAnnotation.NEGEnabled(); neg != tc.negEnabled {
			t.Errorf("Test case %q: Expect EnabledNEG() to be %v, be got %v", tc.desc, tc.negEnabled, neg)
		}

		if ing := negAnnotation.NEGEnabledForIngress(); ing != tc.ingress {
			t.Errorf("Test case %q: Expect NEGEnabledForIngress() = %v; want %v", tc.desc, tc.ingress, ing)
		}

		if exposed := negAnnotation.NEGExposed(); exposed != tc.exposed {
			t.Errorf("Test case %q: Expect NEGExposed() = %v; want %v", tc.desc, tc.exposed, exposed)
		}
	}
}

func TestService(t *testing.T) {
	for _, tc := range []struct {
		svc             *v1.Service
		appProtocolsErr bool
		appProtocols    map[string]AppProtocol
		http2           bool
	}{
		{
			svc:          &v1.Service{},
			appProtocols: map[string]AppProtocol{},
		},
		{
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						GoogleServiceApplicationProtocolKey: `{"80": "HTTP", "443": "HTTPS"}`,
					},
				},
			},
			appProtocols: map[string]AppProtocol{"80": "HTTP", "443": "HTTPS"},
		},
		{
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						GoogleServiceApplicationProtocolKey: `{"80": "HTTP", "443": "HTTPS"}`,
						ServiceApplicationProtocolKey:       `{"81": "HTTP", "444": "HTTPS"}`,
					},
				},
			},
			appProtocols: map[string]AppProtocol{"81": "HTTP", "444": "HTTPS"},
		},
		{
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ServiceApplicationProtocolKey: `{"80": "HTTP", "443": "HTTPS"}`,
					},
				},
			},
			appProtocols: map[string]AppProtocol{"80": "HTTP", "443": "HTTPS"},
		},
		{
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ServiceApplicationProtocolKey: `{"443": "HTTP2"}`,
					},
				},
			},
			appProtocols:    map[string]AppProtocol{"443": "HTTP2"},
			appProtocolsErr: true, // Without the http2 flag enabled, expect error
		},
		{
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ServiceApplicationProtocolKey: `{"443": "HTTP2"}`,
					},
				},
			},
			appProtocols: map[string]AppProtocol{"443": "HTTP2"},
			http2:        true,
		},
		{
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ServiceApplicationProtocolKey: `invalid`,
					},
				},
			},
			appProtocolsErr: true,
		},
		{
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ServiceApplicationProtocolKey: `{"SSH": "22"}`,
					},
				},
			},
			appProtocolsErr: true,
		},
	} {
		flags.F.Features.Http2 = tc.http2
		svc := FromService(tc.svc)
		ap, err := svc.ApplicationProtocols()
		if tc.appProtocolsErr {
			if err == nil {
				t.Errorf("for service %+v; svc.ApplicationProtocols() = _, %v; want _, error", tc.svc, err)
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(ap, tc.appProtocols) {
			t.Errorf("for service %+v; svc.ApplicationProtocols() = %v, %v; want %v, nil", tc.svc, ap, err, tc.appProtocols)
		}
	}
}

func TestBackendConfigs(t *testing.T) {
	testcases := []struct {
		desc            string
		svc             *v1.Service
		expectedConfigs *BackendConfigs
		expectedErr     error
	}{
		{
			desc:        "no backendConfig annotation",
			svc:         &v1.Service{},
			expectedErr: ErrBackendConfigAnnotationMissing,
		},
		{
			desc: "single backendConfig",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						BackendConfigKey: `{"ports":{"http": "config-http"}}`,
					},
				},
			},
			expectedConfigs: &BackendConfigs{
				Ports: map[string]string{
					"http": "config-http",
				},
			},
		},
		{
			desc: "multiple backendConfigs",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						BackendConfigKey: `{"ports":{"http": "config-http", "https": "config-https"}}`,
					},
				},
			},
			expectedConfigs: &BackendConfigs{
				Ports: map[string]string{
					"http":  "config-http",
					"https": "config-https",
				},
			},
		}, {
			desc: "multiple backendConfigs with default",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						BackendConfigKey: `{"default": "config-default", "ports":{"http": "config-http", "https": "config-https"}}`,
					},
				},
			},
			expectedConfigs: &BackendConfigs{
				Default: "config-default",
				Ports: map[string]string{
					"http":  "config-http",
					"https": "config-https",
				},
			},
		},
		{
			desc: "invalid backendConfig annotation",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						BackendConfigKey: `invalid`,
					},
				},
			},
			expectedErr: ErrBackendConfigInvalidJSON,
		},
		{
			desc: "wrong field name in backendConfig annotation",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						BackendConfigKey: `{"portstypo":{"https": "config-https"}}`,
					},
				},
			},
			expectedErr: ErrBackendConfigNoneFound,
		},
	}

	for _, tc := range testcases {
		svc := FromService(tc.svc)
		configs, err := svc.GetBackendConfigs()
		if !reflect.DeepEqual(configs, tc.expectedConfigs) || tc.expectedErr != err {
			t.Errorf("%s: for annotations %+v; svc.GetBackendConfigs() = %v, %v; want %v, %v", tc.desc, svc.v, configs, err, tc.expectedConfigs, tc.expectedErr)
		}
	}
}
