# GN-WAAS Field Officer Mobile App

React Native (Expo) mobile application for GWL field officers conducting meter audits.

## Features

### Authentication
- Email/password login via GN-WAAS API Gateway
- Biometric authentication (Face ID / Fingerprint)
- JWT stored in Expo SecureStore (encrypted)

### Job Management
- View today assigned audit jobs sorted by priority
- Pull-to-refresh, job status tracking

### Meter Capture (Core Feature)
1. GPS Fence Check - Verifies officer is within 50m of meter
2. Camera Capture - Full-screen camera with meter alignment guide
3. OCR Processing - Photo sent to OCR service (Tesseract)
4. Review and Confirm - Officer verifies OCR result or enters manually
5. Immutable Submission - Photo hash + GPS + reading submitted to audit trail

### SOS Button
- One-tap emergency alert with GPS coordinates to supervisor
- Vibration + haptic feedback

## Tech Stack
- Expo 52 + React Native 0.76
- expo-camera, expo-location, expo-secure-store, expo-local-authentication
- TanStack Query, Zustand, Axios

## Build
    npm install
    npm run android
    eas build --platform android --profile production
