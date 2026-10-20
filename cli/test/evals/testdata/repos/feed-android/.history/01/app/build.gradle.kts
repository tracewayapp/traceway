plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("com.tracewayapp.symbols") version "1.0.1"
}

android {
    namespace = "com.example.pinboard"
    compileSdk = 35
    defaultConfig {
        applicationId = "com.example.pinboard"
        minSdk = 26
        targetSdk = 35
        buildConfigField("String", "TRACEWAY_DSN", "\"${providers.gradleProperty("tracewayDsn").getOrElse("")}\"")
        versionCode = 5702
        versionName = "5.7.2"
    }
    buildFeatures {
        buildConfig = true
    }
    buildTypes {
        release {
            isMinifyEnabled = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }
}

dependencies {
    implementation("com.tracewayapp:traceway:1.0.1")
    implementation("androidx.recyclerview:recyclerview:1.4.0")
    implementation("androidx.constraintlayout:constraintlayout:2.2.1")
    implementation("com.squareup.retrofit2:retrofit:2.11.0")
    implementation("com.squareup.retrofit2:converter-gson:2.11.0")
    implementation("io.coil-kt:coil:2.7.0")
}
