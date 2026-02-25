# Flomation Automate - Actions Reference

This document provides a comprehensive reference for all actions available in Flomation Automate. Actions are the building blocks of your automation flows.

---

## Table of Contents

- [Triggers](#triggers)
  - [Manual Trigger](#manual-trigger)
- [AWS Actions](#aws-actions)
  - [AWS EC2](#aws-ec2)
  - [AWS S3](#aws-s3)
- [Communication Actions](#communication-actions)
  - [Send SMTP Email](#send-smtp-email)
- [Flow Control](#flow-control)
  - [Set Output](#set-output)

---

## Triggers

### Manual Trigger

**Action ID:** `trigger/manual`

A simple trigger that is invoked by pressing the play button or serves as the default entry point for flow execution.

**Details:**
- **Author:** Andy Esser
- **Organisation:** Flomation
- **Last Updated:** 27/11/2025

**Use Cases:**
- Starting a flow manually from the UI
- Testing and debugging flows
- Default trigger for flows without other triggers

---

## AWS Actions

### AWS EC2

#### AWS EC2 Describe

**Action ID:** `aws/ec2/describe`

Retrieve information about AWS EC2 instances in your account.

**Details:**
- **Author:** Andy Esser
- **Organisation:** Flomation
- **Last Updated:** 27/11/2025

**Use Cases:**
- Listing EC2 instances
- Monitoring instance states
- Inventory management

---

### AWS S3

#### AWS S3 Get Object

**Action ID:** `aws/s3/get`

Retrieve objects (files) from an AWS S3 bucket.

**Details:**
- **Author:** Andy Esser
- **Organisation:** Flomation
- **Last Updated:** 03/01/2026

**Use Cases:**
- Download files from S3 storage
- Process data stored in S3 buckets
- Retrieve configuration files

---

#### AWS S3 Put

**Action ID:** `aws/s3/put`

Upload objects (files) to an AWS S3 bucket.

**Details:**
- **Author:** Andy Esser
- **Organisation:** Flomation
- **Last Updated:** 27/11/2025

**Use Cases:**
- Upload processed files to S3
- Backup data to cloud storage
- Store automation outputs

---

#### AWS S3 Delete

**Action ID:** `aws/s3/delete`

Delete objects (files) from an AWS S3 bucket.

**Details:**
- **Author:** Andy Esser
- **Organisation:** Flomation
- **Last Updated:** 27/11/2025

**Use Cases:**
- Clean up temporary files
- Remove outdated data
- Manage storage costs

---

## Communication Actions

### Send SMTP Email

**Action ID:** `common/smtp`

Send HTML emails via SMTP protocol with TLS support.

**Details:**
- **Author:** Andy Esser
- **Organisation:** Flomation
- **Last Updated:** 27/11/2025

**Inputs:**

| Input | Type | Description |
|-------|------|-------------|
| **to** | String | Recipient email address |
| **from** | String | Sender email address |
| **subject** | String | Email subject line |
| **message** | String | HTML email content/body |
| **smtp_host** | String | SMTP server hostname |
| **smtp_username** | String | SMTP authentication username |
| **smtp_password** | String | SMTP authentication password |
| **smtp_port** | Integer | SMTP server port (typically 587 for TLS) |
| **smtp_secure** | Boolean | Enable TLS encryption |

**Use Cases:**
- Sending notification emails
- Alerting on automation events
- Delivering reports via email
- User communication workflows

**Example Configuration:**
- SMTP Host: `smtp.gmail.com`
- SMTP Port: `587`
- SMTP Secure: `true`

---

## Flow Control

### Set Output

**Action ID:** `output/set`

Pass data out from a flow, making it available to external systems or parent flows.

**Details:**
- **Author:** Andy Esser
- **Organisation:** Flomation
- **Last Updated:** 27/11/2025

**Use Cases:**
- Returning results from a flow
- Passing data to calling systems
- Exporting processed information
- Integration with external applications

---

## Additional Information

### Action Identifiers

Each action has a unique identifier (Action ID) that follows the pattern `category/subcategory/action`. Use these identifiers when:
- Registering actions programmatically
- Referencing actions in flow JSON files
- Debugging action execution

### Getting Help

For more information about Flomation Automate:
- **Website:** [https://www.flomation.co](https://www.flomation.co)
- **Support:** Contact Flomation support team

---

*Generated from actions-manifest.json*