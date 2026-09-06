# Privacy Policy

**Last Updated:** January 20, 2026

## Introduction

Eterno Mail ("we," "our," or "the Application") is an open-source desktop email client. The official Eterno Mail website is [https://app.weslleys.com/](https://app.weslleys.com/). This Privacy Policy explains how Eterno Mail handles your information. Because Eterno Mail is a desktop application that runs entirely on your computer, our approach to privacy is fundamentally different from cloud-based services.

**Key Point:** Eterno Mail does not operate any servers that collect, store, or process your data. All your information stays on your device.

## Information We Do Not Collect

Unlike many applications, Eterno Mail:

- **Does not collect** any personal information on external servers
- **Does not transmit** your emails, contacts, or credentials to us
- **Does not use** analytics, telemetry, or tracking of any kind
- **Does not display** advertisements
- **Does not sell or share** any user data (we don't have access to it)

## Information Stored Locally on Your Device

Eterno Mail stores the following information locally on your computer:

### Email Data
- Email messages synchronized from your email accounts
- Folder structure and organization
- Attachments (cached locally)
- Draft messages

### Account Credentials
- Email account configurations (server addresses, ports)
- Authentication tokens (OAuth tokens for Gmail, Outlook)
- App-specific passwords (stored in your system's secure keyring)

### Contacts
- Contact information synchronized from CardDAV, Google, or Microsoft accounts
- Used solely for email address autocomplete

### Application Settings
- Your preferences and settings
- Window size and position
- Theme preferences

**All of this data is stored in your local application data directory and never leaves your device except when communicating directly with your email providers.**

## Google API Services

Eterno Mail uses Google APIs to provide Gmail integration. Our use of information received from Google APIs adheres to the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including the Limited Use requirements.

### Google API Scopes Used

- **IMAP/SMTP Access** - To read, send, and manage your emails via standard IMAP/SMTP protocols (OAuth2 authenticated)
- **Google People API** - To access your contacts for email autocomplete

### Limited Use Disclosure

Eterno Mail's use and transfer of information received from Google APIs to any other app will adhere to the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including the Limited Use requirements.

Specifically:
- Eterno Mail only uses Google data to provide email functionality to you
- Eterno Mail does not transfer Google data to third parties
- Eterno Mail does not use Google data for advertising
- Eterno Mail does not use Google data to train AI/ML models
- Human access to Google data is limited to debugging with user consent

## Microsoft API Services

Eterno Mail uses Microsoft Graph APIs to provide Outlook/Microsoft 365 integration.

### Microsoft API Scopes Used

- **Mail.ReadWrite** - To read, send, and manage your emails
- **Contacts.Read** - To access your contacts for email autocomplete
- **User.Read** - To access your basic profile information
- **offline_access** - To maintain your session

All Microsoft data is stored locally and used solely for email functionality.

## Data Security

Your data security depends on:

1. **Your device security** - Eterno Mail stores data locally, so your device's security measures (disk encryption, password protection, etc.) protect your email data
2. **Secure connections** - All connections to email servers use TLS/SSL encryption
3. **System keyring** - Sensitive credentials are stored in your operating system's secure credential storage
4. **No cloud exposure** - Your data is never transmitted to our servers because we don't operate any

## Third-Party Services

Eterno Mail only connects to services you explicitly configure:

- Your email provider (via IMAP/SMTP)
- Google APIs (if you use Gmail)
- Microsoft APIs (if you use Outlook/Microsoft 365)
- CardDAV servers (if you configure contact sync)

We do not integrate with any advertising networks, analytics services, or other third-party data collectors.

## Data Retention and Deletion

Since all data is stored locally on your device:

- **You control retention** - Data remains until you delete it
- **Uninstalling Eterno Mail** - Removes the application but may leave data in your local application data directory
- **Complete removal** - Delete the Eterno Mail data directory to remove all locally stored data

## Children's Privacy

Eterno Mail is not directed at children under 13. We do not knowingly collect information from children. Since Eterno Mail stores all data locally and does not transmit data to external servers, parental supervision of the device itself is the appropriate safeguard.

## Changes to This Privacy Policy

We may update this Privacy Policy from time to time. Changes will be reflected in the "Last Updated" date. Since Eterno Mail is open source, you can review any changes in our public repository.

## Open Source Transparency

Eterno Mail is open-source software. You can review exactly how we handle your data by examining the [Eterno Mail source code](https://github.com/wesleiaqui/EternoMail).

## Contact Us

If you have questions about this Privacy Policy, please use:

Use the [Eterno Mail website](https://app.weslleys.com/) or [project repository](https://github.com/wesleiaqui/EternoMail).
