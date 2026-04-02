import org.springframework.boot.gradle.tasks.run.BootRun

plugins {
    id("java")
    id("org.springframework.boot") version "3.2.0"
    id("io.spring.dependency-management") version "1.1.4"
}

group = "hj320.network_tool"
version = "1.0-SNAPSHOT"

repositories {
    mavenCentral()
}

java {
    sourceCompatibility = JavaVersion.VERSION_21
    targetCompatibility = JavaVersion.VERSION_21
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(21))
    }
}

// This tells Spring Boot what the main class is
springBoot {
    mainClass.set("hj320.network_tool.Main")
}

// Configure bootRun to use Java 21 toolchain and preview features if needed
tasks.named<BootRun>("bootRun") {
    mainClass.set("hj320.network_tool.Main")
    jvmArgs = listOf("--enable-preview") // optional
}

dependencies {
    implementation("org.springframework.boot:spring-boot-starter-web")
//    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    implementation("com.fasterxml.jackson.core:jackson-databind:2.16.2")
    implementation("org.xerial:sqlite-jdbc:3.41.2.1")
    implementation("mysql:mysql-connector-java:8.0.33")
    implementation("org.springframework.boot:spring-boot-starter")

    testImplementation(platform("org.junit:junit-bom:5.10.0"))
    testImplementation("org.junit.jupiter:junit-jupiter")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
    testImplementation("org.springframework.boot:spring-boot-starter-test")

}

tasks.test {
    useJUnitPlatform()
}