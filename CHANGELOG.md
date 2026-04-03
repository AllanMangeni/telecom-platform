# Changelog

All notable changes to the TaaS Platform will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-04-01

### Added
- Initial release of TaaS Platform
- API Server (Go 1.26) with RESTful endpoints
- Carrier Connector for GSMA ES2+ integration
- Charging Engine (Rust 1.94) with Redis-based credit control
- Packet Gateway stub with eBPF architecture
- Web Dashboard (Next.js 15) with Tailwind CSS
- Docker deployment with docker-compose
- Kubernetes manifests for production deployment
- Complete documentation (API, Architecture, Deployment)
- Setup scripts for MongoDB and Redis
- Development environment setup script
- Comprehensive TODO list with research links

### Features
- eSIM creation and management
- Real-time credit control
- Usage tracking and reporting
- Developer dashboard
- Webhook support (planned)
- Multi-tenant architecture
- Horizontal scaling support

### Technical
- Go 1.26 with Green Tea GC
- Rust 1.94 with array_windows
- TypeScript/Next.js 15
- MongoDB 7.0 for subscriber data
- Redis 7 for real-time state
- free5GC integration ready
- eBPF/Aya framework integration

## [Unreleased]

### Planned Features
- Complete eBPF packet gateway implementation
- Real-time webhook delivery
- Advanced analytics dashboard
- Multi-region deployment support
- SGP.32 (IoT eSIM) support
- CLI tool for developers
- Terraform provider
- Enhanced monitoring and alerting

---

## Version History

- **1.0.0** - Initial release with core functionality
