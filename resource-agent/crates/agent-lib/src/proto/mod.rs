//! Generated protobuf code
//!
//! This module contains the generated Rust code from protobuf definitions.
//! The code is generated at build time by tonic-build.
//!
//! If protoc is not available, stub types are provided for development.

#[cfg(feature = "proto-gen")]
pub mod predictor {
    pub mod v1 {
        tonic::include_proto!("predictor.v1");
    }
}

// Provide stub types when proto generation is not available
#[cfg(not(feature = "proto-gen"))]
pub mod predictor {
    pub mod v1 {
        use prost::Message;

        #[derive(Clone, PartialEq, Message)]
        pub struct RegisterRequest {
            #[prost(string, tag = "1")]
            pub agent_id: String,
            #[prost(string, tag = "2")]
            pub node_name: String,
            #[prost(string, tag = "3")]
            pub kubernetes_version: String,
            #[prost(string, tag = "4")]
            pub agent_version: String,
            #[prost(string, tag = "5")]
            pub model_version: String,
        }

        #[derive(Clone, PartialEq, Message)]
        pub struct RegisterResponse {
            #[prost(bool, tag = "1")]
            pub success: bool,
            #[prost(string, tag = "2")]
            pub message: String,
            #[prost(message, optional, tag = "3")]
            pub config: Option<AgentConfig>,
        }

        #[derive(Clone, PartialEq, Message)]
        pub struct AgentConfig {
            #[prost(int32, tag = "1")]
            pub collection_interval_seconds: i32,
            #[prost(int32, tag = "2")]
            pub prediction_interval_seconds: i32,
            #[prost(int32, tag = "3")]
            pub sync_interval_seconds: i32,
            #[prost(bool, tag = "4")]
            pub anomaly_detection_enabled: bool,
        }

        #[derive(Clone, PartialEq, Message)]
        pub struct SyncMetricsRequest {
            #[prost(string, tag = "1")]
            pub agent_id: String,
            #[prost(string, tag = "2")]
            pub node_name: String,
            #[prost(message, optional, tag = "3")]
            pub timestamp: Option<prost_types::Timestamp>,
            #[prost(message, repeated, tag = "4")]
            pub metrics: Vec<ContainerMetrics>,
            #[prost(message, repeated, tag = "5")]
            pub predictions: Vec<ResourceProfile>,
            #[prost(message, repeated, tag = "6")]
            pub anomalies: Vec<Anomaly>,
        }

        // Type alias for backward compatibility
        pub type MetricsBatch = SyncMetricsRequest;

        #[derive(Clone, PartialEq, Message)]
        pub struct ContainerMetrics {
            #[prost(string, tag = "1")]
            pub container_id: String,
            #[prost(string, tag = "2")]
            pub pod_name: String,
            #[prost(string, tag = "3")]
            pub namespace: String,
            #[prost(string, tag = "4")]
            pub deployment: String,
            #[prost(message, optional, tag = "5")]
            pub timestamp: Option<prost_types::Timestamp>,
            #[prost(float, tag = "6")]
            pub cpu_usage_cores: f32,
            #[prost(uint64, tag = "7")]
            pub cpu_throttled_periods: u64,
            #[prost(uint64, tag = "8")]
            pub cpu_throttled_time_ns: u64,
            #[prost(uint64, tag = "9")]
            pub memory_usage_bytes: u64,
            #[prost(uint64, tag = "10")]
            pub memory_working_set_bytes: u64,
            #[prost(uint64, tag = "11")]
            pub memory_cache_bytes: u64,
            #[prost(uint64, tag = "12")]
            pub memory_rss_bytes: u64,
            #[prost(uint64, tag = "13")]
            pub network_rx_bytes: u64,
            #[prost(uint64, tag = "14")]
            pub network_tx_bytes: u64,
        }

        #[derive(Clone, PartialEq, Message)]
        pub struct ResourceProfile {
            #[prost(string, tag = "1")]
            pub container_id: String,
            #[prost(string, tag = "2")]
            pub pod_name: String,
            #[prost(string, tag = "3")]
            pub namespace: String,
            #[prost(string, tag = "4")]
            pub deployment: String,
            #[prost(uint32, tag = "5")]
            pub cpu_request_millicores: u32,
            #[prost(uint32, tag = "6")]
            pub cpu_limit_millicores: u32,
            #[prost(uint64, tag = "7")]
            pub memory_request_bytes: u64,
            #[prost(uint64, tag = "8")]
            pub memory_limit_bytes: u64,
            #[prost(float, tag = "9")]
            pub confidence: f32,
            #[prost(string, tag = "10")]
            pub model_version: String,
            #[prost(message, optional, tag = "11")]
            pub generated_at: Option<prost_types::Timestamp>,
            #[prost(int32, tag = "12")]
            pub time_window: i32,
        }

        #[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, Default)]
        #[repr(i32)]
        pub enum TimeWindow {
            #[default]
            Unspecified = 0,
            Peak = 1,
            OffPeak = 2,
            Weekly = 3,
        }

        impl TimeWindow {
            pub fn as_str_name(&self) -> &'static str {
                match self {
                    TimeWindow::Unspecified => "TIME_WINDOW_UNSPECIFIED",
                    TimeWindow::Peak => "TIME_WINDOW_PEAK",
                    TimeWindow::OffPeak => "TIME_WINDOW_OFF_PEAK",
                    TimeWindow::Weekly => "TIME_WINDOW_WEEKLY",
                }
            }
        }

        #[derive(Clone, PartialEq, Message)]
        pub struct Anomaly {
            #[prost(string, tag = "1")]
            pub container_id: String,
            #[prost(string, tag = "2")]
            pub pod_name: String,
            #[prost(string, tag = "3")]
            pub namespace: String,
            #[prost(int32, tag = "4")]
            pub r#type: i32,
            #[prost(int32, tag = "5")]
            pub severity: i32,
            #[prost(string, tag = "6")]
            pub message: String,
            #[prost(message, optional, tag = "7")]
            pub detected_at: Option<prost_types::Timestamp>,
        }

        #[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, Default)]
        #[repr(i32)]
        pub enum AnomalyType {
            #[default]
            Unspecified = 0,
            MemoryLeak = 1,
            CpuSpike = 2,
            OomRisk = 3,
        }

        #[derive(Clone, Copy, Debug, PartialEq, Eq, Hash, PartialOrd, Ord, Default)]
        #[repr(i32)]
        pub enum Severity {
            #[default]
            Unspecified = 0,
            Warning = 1,
            Critical = 2,
        }

        #[derive(Clone, PartialEq, Message)]
        pub struct SyncMetricsResponse {
            #[prost(bool, tag = "1")]
            pub success: bool,
            #[prost(string, tag = "2")]
            pub message: String,
            #[prost(int64, tag = "3")]
            pub metrics_received: i64,
            #[prost(int64, tag = "4")]
            pub predictions_received: i64,
        }

        // Type alias for backward compatibility
        pub type SyncResponse = SyncMetricsResponse;

        #[derive(Clone, PartialEq, Message)]
        pub struct GetModelUpdateRequest {
            #[prost(string, tag = "1")]
            pub agent_id: String,
            #[prost(string, tag = "2")]
            pub current_model_version: String,
        }

        // Type alias for backward compatibility
        pub type ModelRequest = GetModelUpdateRequest;

        #[derive(Clone, PartialEq, Message)]
        pub struct GetModelUpdateResponse {
            #[prost(bool, tag = "1")]
            pub update_available: bool,
            #[prost(string, tag = "2")]
            pub new_version: String,
            #[prost(bytes = "vec", tag = "3")]
            pub model_weights: Vec<u8>,
            #[prost(string, tag = "4")]
            pub checksum: String,
            #[prost(message, optional, tag = "5")]
            pub metadata: Option<ModelMetadata>,
        }

        // Type alias for backward compatibility
        pub type ModelResponse = GetModelUpdateResponse;

        #[derive(Clone, PartialEq, Message)]
        pub struct ModelMetadata {
            #[prost(string, tag = "1")]
            pub version: String,
            #[prost(message, optional, tag = "2")]
            pub created_at: Option<prost_types::Timestamp>,
            #[prost(float, tag = "3")]
            pub validation_accuracy: f32,
            #[prost(int64, tag = "4")]
            pub size_bytes: i64,
        }

        #[derive(Clone, PartialEq, Message)]
        pub struct UploadGradientsRequest {
            #[prost(string, tag = "1")]
            pub agent_id: String,
            #[prost(string, tag = "2")]
            pub model_version: String,
            #[prost(bytes = "vec", tag = "3")]
            pub gradients: Vec<u8>,
            #[prost(int64, tag = "4")]
            pub sample_count: i64,
        }

        // Type alias for backward compatibility
        pub type GradientsRequest = UploadGradientsRequest;

        #[derive(Clone, PartialEq, Message)]
        pub struct UploadGradientsResponse {
            #[prost(bool, tag = "1")]
            pub success: bool,
            #[prost(string, tag = "2")]
            pub message: String,
        }

        // Type alias for backward compatibility
        pub type GradientsResponse = UploadGradientsResponse;

        pub mod predictor_sync_service_client {
            use super::*;
            use tonic::codegen::*;
            use tonic::transport::Uri;

            #[derive(Debug, Clone)]
            pub struct PredictorSyncServiceClient<T> {
                inner: tonic::client::Grpc<T>,
            }

            impl PredictorSyncServiceClient<tonic::transport::Channel> {
                pub fn new(channel: tonic::transport::Channel) -> Self {
                    let inner = tonic::client::Grpc::new(channel);
                    Self { inner }
                }
            }

            impl<T> PredictorSyncServiceClient<T>
            where
                T: tonic::client::GrpcService<tonic::body::BoxBody>,
                T::Error: Into<StdError>,
                T::ResponseBody: Body<Data = Bytes> + Send + 'static,
                <T::ResponseBody as Body>::Error: Into<StdError> + Send,
            {
                pub fn with_origin(inner: T, origin: Uri) -> Self {
                    let inner = tonic::client::Grpc::with_origin(inner, origin);
                    Self { inner }
                }

                pub fn with_interceptor<F>(
                    inner: T,
                    interceptor: F,
                ) -> PredictorSyncServiceClient<InterceptedService<T, F>>
                where
                    F: tonic::service::Interceptor,
                    T::ResponseBody: Default,
                    T: tonic::codegen::Service<
                        http::Request<tonic::body::BoxBody>,
                        Response = http::Response<
                            <T as tonic::client::GrpcService<tonic::body::BoxBody>>::ResponseBody,
                        >,
                    >,
                    <T as tonic::codegen::Service<http::Request<tonic::body::BoxBody>>>::Error:
                        Into<StdError> + Send + Sync,
                {
                    let inner = InterceptedService::new(inner, interceptor);
                    let inner = tonic::client::Grpc::new(inner);
                    PredictorSyncServiceClient { inner }
                }

                pub async fn register(
                    &mut self,
                    request: impl tonic::IntoRequest<RegisterRequest>,
                ) -> Result<tonic::Response<RegisterResponse>, tonic::Status> {
                    self.inner.ready().await.map_err(|e| {
                        tonic::Status::new(
                            tonic::Code::Unknown,
                            format!("Service was not ready: {}", e.into()),
                        )
                    })?;
                    let codec = tonic::codec::ProstCodec::default();
                    let path = http::uri::PathAndQuery::from_static(
                        "/predictor.v1.PredictorSyncService/Register",
                    );
                    self.inner.unary(request.into_request(), path, codec).await
                }

                pub async fn sync_metrics(
                    &mut self,
                    request: impl tonic::IntoStreamingRequest<Message = SyncMetricsRequest>,
                ) -> Result<tonic::Response<SyncMetricsResponse>, tonic::Status> {
                    self.inner.ready().await.map_err(|e| {
                        tonic::Status::new(
                            tonic::Code::Unknown,
                            format!("Service was not ready: {}", e.into()),
                        )
                    })?;
                    let codec = tonic::codec::ProstCodec::default();
                    let path = http::uri::PathAndQuery::from_static(
                        "/predictor.v1.PredictorSyncService/SyncMetrics",
                    );
                    self.inner
                        .client_streaming(request.into_streaming_request(), path, codec)
                        .await
                }

                pub async fn get_model_update(
                    &mut self,
                    request: impl tonic::IntoRequest<GetModelUpdateRequest>,
                ) -> Result<tonic::Response<GetModelUpdateResponse>, tonic::Status>
                {
                    self.inner.ready().await.map_err(|e| {
                        tonic::Status::new(
                            tonic::Code::Unknown,
                            format!("Service was not ready: {}", e.into()),
                        )
                    })?;
                    let codec = tonic::codec::ProstCodec::default();
                    let path = http::uri::PathAndQuery::from_static(
                        "/predictor.v1.PredictorSyncService/GetModelUpdate",
                    );
                    self.inner.unary(request.into_request(), path, codec).await
                }

                pub async fn upload_gradients(
                    &mut self,
                    request: impl tonic::IntoRequest<UploadGradientsRequest>,
                ) -> Result<tonic::Response<UploadGradientsResponse>, tonic::Status>
                {
                    self.inner.ready().await.map_err(|e| {
                        tonic::Status::new(
                            tonic::Code::Unknown,
                            format!("Service was not ready: {}", e.into()),
                        )
                    })?;
                    let codec = tonic::codec::ProstCodec::default();
                    let path = http::uri::PathAndQuery::from_static(
                        "/predictor.v1.PredictorSyncService/UploadGradients",
                    );
                    self.inner.unary(request.into_request(), path, codec).await
                }
            }
        }

        // Backward compatibility alias
        pub mod predictor_sync_client {
            pub use super::predictor_sync_service_client::PredictorSyncServiceClient as PredictorSyncClient;
        }
    }
}

pub use predictor::v1::predictor_sync_service_client::PredictorSyncServiceClient;
// Backward compatibility alias
pub use predictor::v1::predictor_sync_client::PredictorSyncClient;
pub use predictor::v1::*;
