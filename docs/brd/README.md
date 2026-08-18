# Business Requirements Document

> English version · [Русская версия](i18n/ru/README.md)

| | |
|---|---|
| **Version** | 0.1 |
| **Date** | 2026-06-15 |
| **Status** | Draft |
| **Author** | [Bakin Vladislav Artemovich](https://meidori.tech) |

## Table of Contents

1. [Overview](#1-overview)
2. [Stakeholders](#2-stakeholders)
3. [Project Scope](#3-project-scope)
4. [Current Process Description (AS-IS)](#4-current-process-description-as-is)
5. [Business Requirements](#5-business-requirements)
6. [Requirements and Constraints](#6-requirements-and-constraints)
7. [Risks](#7-risks)
8. [Acceptance Criteria](#8-acceptance-criteria)
9. [Company Expenses](#9-company-expenses)

---

## 1 Overview

**What are we building?** A CRM to automate the full cycle of property appraisal: from receiving a client request to delivering the completed report.

Figures 1–3 illustrate the business processes being automated.

> Figure 1 – BPMN: Appraisal Request Formation  
> Figure 2 – BPMN: Inspection Process  
> Figure 3 – BPMN: Appraisal Process

The company performs property appraisal in three stages: receiving a request, conducting an on-site inspection, and generating an appraisal report. Currently, client interaction happens via phone and email, data is stored in a fragmented manner, and coordination between the appraiser and inspector is entirely manual.

**Project goal** – build a CRM that consolidates the entire cycle into a single platform: from the client submitting a request to receiving the completed report.

**Expected outcomes:**

- Reduced processing time per request
- Elimination of request loss due to manual handoffs
- Centralized data storage
- Transparent request status visibility for the client at every stage
- Structured storage of inspection data and appraisal reports

---

## 2 Stakeholders

**Table 1 – Stakeholders**

| Role | Who | Interest in the Product |
|------|-----|------------------------|
| Client | Individual or legal entity | Submit a request, track status, receive the report |
| Appraiser | Company employee | Accept requests, conduct appraisals, send reports |
| Inspector | Company employee | Receive assignments, conduct inspections, upload materials |
| Administrator | Company employee | Maintain system operation |

---

## 3 Project Scope

**In scope:**

- Client personal account (request submission, status tracking, report download)
- Appraiser interface (request processing, appraisal, report delivery)
- Inspector interface (assignment retrieval, photo and inspection data upload)
- Admin Panel (user management, request management, inspector assignment)
- Storage of inspection photographs
- Report generation and storage
- Responsive web application for any device
- Status change notifications for all participants

**Out of scope:**

- Integration with external registries (USRN, Rosreestr)
- Mobile application
- Desktop application
- Automatic comparable property search via external sources
- Electronic signature of reports
- Payment processing for company services

---

## 4 Current Process Description (AS-IS)

### 4.1 Request Formation

The client calls or emails the company. The appraiser manually records data and interviews the client over the phone. Data is stored in a non-centralized manner.

### 4.2 Inspection Process

The appraiser contacts the client to schedule a time. If the client can be present, the inspector travels to the site. If not, the process is paused until a call can be arranged. After the inspection, photos and data are sent to the appraiser directly.

### 4.3 Appraisal Process

The appraiser receives the property data, manually searches for comparable properties, conducts the appraisal, generates the report, and sends it to the client.

### Open Questions

- Description of the business process "Client interview during request formation"
- Description of the business process "Property appraisal"
- How is property data currently stored?

### Key AS-IS Pain Points

- No single entry point for the client
- Request status is opaque
- Coordination between all process participants is manual
- No unified data storage system

---

## 5 Business Requirements

### 5.1 Request Management

| ID | Requirement |
|----|-------------|
| BR-001 | The client must be able to submit a request via the web interface, specifying the property type, address, and contact details, without calling the company. |
| BR-001a | The client must be able to self-register on the platform via the web interface (Keycloak self-registration or client portal sign-up), providing email, phone number, and password. |
| BR-002 | When a new request is received, the appraiser must receive a notification (email). |
| BR-003 | If an unregistered/guest client submits a request, the system must automatically register a client account in Keycloak using their contact email, link the new request to this account, and send temporary credentials / a password setup link to their email. |
| BR-004 | A request must follow a status model reflecting the current stage: **New → In Progress → Inspection Scheduled → Inspection Completed → Appraisal → Report Sent → Closed**. |
| BR-005 | The client must be able to view the current status of their request in their personal account without needing to call. |

### 5.2 Inspection Process

| ID | Requirement |
|----|-------------|
| BR-006 | The appraiser must be able to assign an inspector to a request via the system interface. |
| BR-007 | The inspector must receive an inspection assignment containing property and client details. |
| BR-007a | The client must receive information about the assigned inspector. |
| BR-008 | The inspector must be able to upload property photographs that are automatically linked to the request. |
| BR-009 | The system must record the date and outcome of each attempt to contact the client to schedule an inspection time. |

### 5.3 Appraisal Process

| ID | Requirement |
|----|-------------|
| BR-010 | The appraiser must have access to all property data and inspection photographs within a single request. |
| BR-011 | The system must support entry of comparable property data and appraisal calculation (specific formulas — defined in a separate Function Spec). |
| BR-012 | The appraiser must be able to generate and send the report to the client through the system. |
| BR-013 | The client must receive a notification about the completed report and be able to download it in their personal account. |

### 5.4 User Management

| ID | Requirement |
|----|-------------|
| BR-014 | The system must support a role model: Client, Appraiser, Inspector, Administrator. |
| BR-015 | The administrator must be able to create, block, and edit employee and client accounts. |
| BR-016 | Access to request data must be role-restricted: clients see only their own requests; inspectors see only their assigned inspections. |

### Open Questions

- **BR-002.** A baseline questionnaire for the "Client interview" sub-process may need to be defined.
- **BR-002.** Alternative notification channels for the appraiser (beyond email) may be considered.

---

## 6 Requirements and Constraints

**Assumptions:**

- Clients have internet access and can use the web interface.
- Company employees have internet access and can use the web interface.

**Constraints:**

The technology stack is fixed:

| Layer | Technology |
|-------|------------|
| Backend | Go |
| Database | PostgreSQL |
| Cache | Redis |
| Message Broker | Apache Kafka |
| Authentication | Keycloak |
| Search | TypeSense *(under consideration)* |
| Frontend | TypeScript, React |
| File Storage | S3 Yandex Cloud |

- All data (requests, inspection photos, appraisal reports) is retained for **5 years**.
- The company must provide a server for deployment and cover S3 Yandex Cloud costs (to be estimated based on 5-year retention volume).
- All source code will remain open-source under the **Apache 2.0** license.

---

## 7 Risks

| ID | Risk | Probability | Impact | Mitigation |
|----|------|-------------|--------|------------|
| R-001 | Appraisal calculation formulas are not formalized | High | Critical | Define formulas in a Function Spec before development of the appraisal service begins |
| R-002 | Clients do not adopt the web interface | Medium | High | Simple onboarding, email notifications with a link to the personal account |
| R-003 | Inspector cannot conveniently upload photos from a phone | Medium | Medium | Responsive interface, mandatory testing on mobile devices |
| R-004 | Employees resist changing their established workflow | Medium | High | Involve employees in interface review during the Figma design phase |
| R-005 | High S3 storage costs due to photo volume over 5 years | Low | Medium | Estimate storage volume before deployment and set per-request upload limits |

---

## 8 Acceptance Criteria

### Request Submission
- The client submits a request via the web interface without calling the company.
- The appraiser receives a new request notification within 1 minute.

### Request Management
- The appraiser assigns an inspector via the system interface.
- The request status changes automatically at each stage transition.

### Inspection
- The inspector uploads property photographs via the web interface from a mobile device.
- Photographs appear in the request card visible to the appraiser.

### Appraisal
- The appraiser generates and sends the report through the system.
- The client receives a notification and can download the report in their personal account.

### Role Model
- The client sees only their own requests.
- The inspector sees only their assigned inspections.
- The appraiser has access to all requests.

### Performance
- Page load time does not exceed 3 seconds on a standard internet connection.

### Deployment
- The system is deployed on the client's server, accessible via HTTPS, with a configured CI/CD pipeline.

---

## 9 Company Expenses

Main ongoing costs:

- Server and domain hosting (if the company does not have its own)
- S3 bucket storage (if the company does not have its own storage)

These are recurring monthly/annual expenses. Exact pricing will be confirmed after the data storage policy is agreed upon.
