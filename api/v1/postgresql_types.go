/*
Copyright 2022.

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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresqlSpec defines the desired state of Postgresql
type PostgresqlSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	DefaultUser string `json:"defaultuser"`

	Password string `json:"password"`
}

type PgPhase string

const (
	PgUp      PgPhase = "up"
	PgPending PgPhase = "pending"
	PgFailed  PgPhase = "Failed"
)

// PostgresqlStatus defines the observed state of Postgresql
type PostgresqlStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Phase PgPhase `json:"active,omitempty"`

	Active corev1.ObjectReference `json:"active,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Postgresql is the Schema for the postgresqls API
type Postgresql struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlSpec   `json:"spec,omitempty"`
	Status PostgresqlStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PostgresqlList contains a list of Postgresql
type PostgresqlList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Postgresql `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Postgresql{}, &PostgresqlList{})
}
