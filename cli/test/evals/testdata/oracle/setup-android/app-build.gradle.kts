import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("com.tracewayapp.symbols") version "2.1.0"
}

// Traceway settings come from the environment (CI) or the gitignored
// local.properties (a developer's machine), never from the repo.
val localProperties = Properties().apply {
    rootProject.file("local.properties").takeIf { it.exists() }?.inputStream()?.use { load(it) }
}
fun localOrEnv(name: String): String = System.getenv(name) ?: localProperties.getProperty(name) ?: ""

android {
    namespace = "com.example.pinboard"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.example.pinboard"
        minSdk = 24
        targetSdk = 35
        versionCode = 5800
        versionName = "5.8.0"
        buildConfigField("String", "API_BASE_URL", "\"https://api.pinboard.example/\"")
        buildConfigField("String", "TRACEWAY_TOKEN", "\"${localOrEnv("TRACEWAY_TOKEN")}\"")
        buildConfigField("String", "TRACEWAY_URL", "\"${localOrEnv("TRACEWAY_URL")}\"")
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

    compileOptions {
        // minSdk 24: OpenTelemetry Java needs java.time and java.util.function.
        isCoreLibraryDesugaringEnabled = true
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }
}

traceway {
    uploadToken = System.getenv("TRACEWAY_UPLOAD_TOKEN")
    url = localOrEnv("TRACEWAY_URL")
}

dependencies {
    implementation("com.tracewayapp:traceway:2.1.0")
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.5")
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("androidx.recyclerview:recyclerview:1.4.0")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
    implementation("com.squareup.retrofit2:retrofit:2.11.0")
    implementation("com.squareup.retrofit2:converter-gson:2.11.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("io.coil-kt:coil:2.7.0")

    testImplementation("junit:junit:4.13.2")
}
