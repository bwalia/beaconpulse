# Credential Manager finds its Play-services provider by reflection.
-if class androidx.credentials.CredentialManager
-keep class androidx.credentials.playservices.** { *; }
