package ru.guseynovia.sirin

import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import java.security.MessageDigest
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.core.content.FileProvider
import org.json.JSONObject
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL
import kotlin.concurrent.thread

/**
 * Автообновление клиента Sirin через GitHub Releases API.
 *
 * - Проверка: GET https://api.github.com/repos/goodbrat-lab/sirin-vpn/releases/latest
 * - Заголовок: Accept: application/vnd.github+json
 * - Кэш: не чаще 1 раза в 24 часа при автоматической проверке
 * - Ручная проверка из настроек в реальном времени
 * - Скачивание APK и запуск системного диалога обновления через FileProvider
 */
object UpdateChecker {

    private const val GITHUB_API_URL =
        "https://api.github.com/repos/goodbrat-lab/sirin-vpn/releases/latest"
    private const val PREFS_NAME = "sirin_update_checker"
    private const val KEY_LAST_CHECK = "last_check_timestamp"
    private const val KEY_DISMISSED_VERSION = "dismissed_version"
    private const val CHECK_INTERVAL_MS = 24L * 60L * 60L * 1000L // 24 часа
    private const val TIMEOUT_MS = 10_000

    data class ReleaseInfo(
        val tagName: String,
        val versionName: String,
        val downloadUrl: String,
        val fileName: String,
        val sizeBytes: Long,
        val expectedSha256: String? = null
    )

    sealed class CheckStatus {
        object Idle : CheckStatus()
        object Checking : CheckStatus()
        object UpToDate : CheckStatus()
        data class Available(val release: ReleaseInfo) : CheckStatus()
        data class Downloading(val progress: Float, val currentMb: Float, val totalMb: Float) : CheckStatus()
        data class ReadyToInstall(val apkFile: File) : CheckStatus()
        data class Error(val message: String) : CheckStatus()
    }

    val status = mutableStateOf<CheckStatus>(CheckStatus.Idle)
    val pendingDialogRelease = mutableStateOf<ReleaseInfo?>(null)
    val downloadProgress = mutableFloatStateOf(0f)
    val isDownloading = mutableStateOf(false)

    /**
     * Сравнение семантических версий (major.minor.patch).
     * Возвращает true, если remote новее local.
     */
    fun isNewer(remoteVersion: String, localVersion: String): Boolean {
        fun parse(v: String): List<Int> =
            v.trim().removePrefix("v").removePrefix("android-v")
                .split("-")[0]
                .split(".")
                .mapNotNull { it.toIntOrNull() }

        val r = parse(remoteVersion)
        val l = parse(localVersion)
        val maxLen = maxOf(r.size, l.size)

        for (i in 0 until maxLen) {
            val rPart = r.getOrElse(i) { 0 }
            val lPart = l.getOrElse(i) { 0 }
            if (rPart > lPart) return true
            if (rPart < lPart) return false
        }
        return false
    }

    /**
     * Проверка обновлений при старте приложения с соблюдением 24-часового кэша.
     */
    fun checkOnStart(context: Context) {
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        val lastCheck = prefs.getLong(KEY_LAST_CHECK, 0L)
        val now = System.currentTimeMillis()

        if (now - lastCheck < CHECK_INTERVAL_MS) {
            // Кэш валиден — пропускаем автоматическую проверку
            return
        }

        thread(name = "sirin-update-check-start") {
            try {
                val release = fetchLatestRelease()
                prefs.edit().putLong(KEY_LAST_CHECK, now).apply()

                if (release != null && isNewer(release.versionName, BuildConfig.VERSION_NAME)) {
                    val dismissed = prefs.getString(KEY_DISMISSED_VERSION, "")
                    if (dismissed != release.versionName) {
                        status.value = CheckStatus.Available(release)
                        pendingDialogRelease.value = release
                    }
                }
            } catch (e: Throwable) {
                // Тихий лог при автоматической фоновой проверке
                TunnelLog.add("UpdateChecker (фон): ${e.message}")
            }
        }
    }

    /**
     * Принудительная проверка обновлений (вызывается пользователем в настройках).
     */
    fun checkManually(context: Context) {
        status.value = CheckStatus.Checking
        val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

        thread(name = "sirin-update-check-manual") {
            try {
                val release = fetchLatestRelease()
                prefs.edit().putLong(KEY_LAST_CHECK, System.currentTimeMillis()).apply()

                if (release == null) {
                    status.value = CheckStatus.Error("Релиз или APK не найден на GitHub")
                } else if (isNewer(release.versionName, BuildConfig.VERSION_NAME)) {
                    status.value = CheckStatus.Available(release)
                } else {
                    status.value = CheckStatus.UpToDate
                }
            } catch (e: Throwable) {
                TunnelLog.add("UpdateChecker (ручной): ${e.message}")
                val err = e.message ?: "Ошибка сети"
                status.value = CheckStatus.Error(err)
            }
        }
    }

    /**
     * Сохранение версии в «Не напоминать».
     */
    fun dismissVersion(context: Context, version: String) {
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
            .edit()
            .putString(KEY_DISMISSED_VERSION, version)
            .apply()
    }

    /**
     * Запрос к GitHub Releases API.
     */
    private fun fetchLatestRelease(): ReleaseInfo? {
        val url = URL(GITHUB_API_URL)
        val conn = (url.openConnection() as HttpURLConnection).apply {
            requestMethod = "GET"
            connectTimeout = TIMEOUT_MS
            readTimeout = TIMEOUT_MS
            setRequestProperty("Accept", "application/vnd.github+json")
            setRequestProperty("User-Agent", "Sirin-Android-Client")
            instanceFollowRedirects = true
        }

        return try {
            val code = conn.responseCode
            if (code != HttpURLConnection.HTTP_OK) {
                throw IllegalStateException("GitHub HTTP $code")
            }

            val body = conn.inputStream.bufferedReader().use { it.readText() }
            val json = JSONObject(body)
            val tagName = json.optString("tag_name", "")
            val cleanVersion = tagName.removePrefix("v").removePrefix("android-v")

            val assets = json.optJSONArray("assets") ?: return null
            var apkUrl: String? = null
            var apkName: String? = null
            var apkSize: Long = 0L
            var checksumsUrl: String? = null

            for (i in 0 until assets.length()) {
                val asset = assets.getJSONObject(i)
                val name = asset.optString("name", "")
                if (name.endsWith(".apk", ignoreCase = true)) {
                    apkUrl = asset.optString("browser_download_url")
                    apkName = name
                    apkSize = asset.optLong("size", 0L)
                } else if (name == "checksums.txt" || name == "sha256sums.txt") {
                    checksumsUrl = asset.optString("browser_download_url")
                }
            }

            var expectedSha256: String? = null
            if (checksumsUrl != null && apkName != null) {
                try {
                    val csConn = (URL(checksumsUrl).openConnection() as HttpURLConnection).apply {
                        connectTimeout = TIMEOUT_MS
                        readTimeout = TIMEOUT_MS
                        setRequestProperty("User-Agent", "Sirin-Android-Client")
                        instanceFollowRedirects = true
                    }
                    if (csConn.responseCode == HttpURLConnection.HTTP_OK) {
                        val csText = csConn.inputStream.bufferedReader().use { it.readText() }
                        for (line in csText.lines()) {
                            val parts = line.trim().split(Regex("\\s+"))
                            if (parts.size >= 2 && parts[1].trimStart('*') == apkName) {
                                expectedSha256 = parts[0].lowercase()
                                break
                            }
                        }
                    }
                    csConn.disconnect()
                } catch (e: Throwable) {
                    TunnelLog.add("UpdateChecker: не удалось прочитать checksums.txt (${e.message})")
                }
            }

            if (apkUrl != null && apkName != null) {
                ReleaseInfo(
                    tagName = tagName,
                    versionName = cleanVersion,
                    downloadUrl = apkUrl,
                    fileName = apkName,
                    sizeBytes = apkSize,
                    expectedSha256 = expectedSha256
                )
            } else {
                null
            }
        } finally {
            conn.disconnect()
        }
    }

    /**
     * Скачивание APK и вызов системного установщика через FileProvider.
     */
    fun startDownloadAndInstall(context: Context, release: ReleaseInfo) {
        if (isDownloading.value) return
        isDownloading.value = true
        downloadProgress.floatValue = 0f

        val appCtx = context.applicationContext

        thread(name = "sirin-apk-downloader") {
            try {
                val updatesDir = appCtx.getExternalFilesDir("updates") ?: File(appCtx.cacheDir, "updates")
                if (!updatesDir.exists()) updatesDir.mkdirs()

                val targetFile = File(updatesDir, release.fileName)
                if (targetFile.exists()) targetFile.delete()

                TunnelLog.add("Скачивание APK: ${release.downloadUrl}")

                var currentUrl = release.downloadUrl
                var conn: HttpURLConnection? = null
                var redirects = 0

                // Поддержка перенаправлений (302 Found) на GitHub CDN
                while (redirects < 5) {
                    val url = URL(currentUrl)
                    conn = (url.openConnection() as HttpURLConnection).apply {
                        requestMethod = "GET"
                        connectTimeout = TIMEOUT_MS
                        readTimeout = 30_000
                        setRequestProperty("User-Agent", "Sirin-Android-Client")
                        instanceFollowRedirects = false
                    }
                    val code = conn.responseCode
                    if (code == HttpURLConnection.HTTP_MOVED_PERM ||
                        code == HttpURLConnection.HTTP_MOVED_TEMP ||
                        code == 307 || code == 308) {
                        currentUrl = conn.getHeaderField("Location")
                        conn.disconnect()
                        redirects++
                    } else if (code == HttpURLConnection.HTTP_OK) {
                        break
                    } else {
                        throw IllegalStateException("HTTP $code")
                    }
                }

                val finalConn = conn ?: throw IllegalStateException("Не удалось установить соединение")
                val totalLength = finalConn.contentLengthLong.let { if (it > 0) it else release.sizeBytes }

                finalConn.inputStream.use { input ->
                    FileOutputStream(targetFile).use { output ->
                        val buffer = ByteArray(16 * 1024)
                        var bytesRead: Int
                        var totalRead = 0L

                        while (input.read(buffer).also { bytesRead = it } != -1) {
                            output.write(buffer, 0, bytesRead)
                            totalRead += bytesRead
                            if (totalLength > 0) {
                                val progress = totalRead.toFloat() / totalLength.toFloat()
                                downloadProgress.floatValue = progress
                                val currMb = totalRead / (1024f * 1024f)
                                val allMb = totalLength / (1024f * 1024f)
                                status.value = CheckStatus.Downloading(progress, currMb, allMb)
                            }
                        }
                    }
                }
                finalConn.disconnect()

                TunnelLog.add("APK скачан: ${targetFile.absolutePath} (${targetFile.length()} байт)")

                // 1. Проверка контрольной суммы SHA-256 (если доступна в релизе)
                if (release.expectedSha256 != null) {
                    val actualSha = computeSha256(targetFile)
                    if (!actualSha.equals(release.expectedSha256, ignoreCase = true)) {
                        targetFile.delete()
                        val err = "Ошибка проверки целостности: SHA-256 не совпадает (ожидался ${release.expectedSha256}, вычислен $actualSha)"
                        TunnelLog.add("UpdateChecker: $err")
                        status.value = CheckStatus.Error(err)
                        isDownloading.value = false
                        return@thread
                    }
                    TunnelLog.add("UpdateChecker: SHA-256 успешно подтверждён ($actualSha)")
                }

                // 2. Проверка цифровой подписи APK (сертификат должен совпадать с текущим приложением)
                val signValid = verifyApkSignature(appCtx, targetFile)
                if (!signValid) {
                    targetFile.delete()
                    val err = "Ошибка безопасности: цифровая подпись скачанного APK не совпадает с установленным приложением"
                    TunnelLog.add("UpdateChecker: $err")
                    status.value = CheckStatus.Error(err)
                    isDownloading.value = false
                    return@thread
                }
                TunnelLog.add("UpdateChecker: цифровая подпись APK успешно подтверждена")

                status.value = CheckStatus.ReadyToInstall(targetFile)
                isDownloading.value = false

                // Запуск системного PackageInstaller через FileProvider
                installApk(appCtx, targetFile)

            } catch (e: Throwable) {
                TunnelLog.add("Ошибка скачивания APK: ${e.message}")
                isDownloading.value = false
                status.value = CheckStatus.Error("Сбой загрузки: ${e.message}")
            }
        }
    }

    /**
     * Запуск Intent для установки APK через FileProvider.
     */
    fun installApk(context: Context, apkFile: File) {
        try {
            val authority = "${context.packageName}.fileprovider"
            val uri = FileProvider.getUriForFile(context, authority, apkFile)

            val intent = Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(uri, "application/vnd.android.package-archive")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(intent)
            TunnelLog.add("Запущен системный установщик APK")
        } catch (e: Throwable) {
            TunnelLog.add("Ошибка запуска установщика: ${e.message}")
            status.value = CheckStatus.Error("Не удалось запустить установку: ${e.message}")
        }
    }

    /**
     * Вычисление контрольной суммы SHA-256 файла.
     */
    fun computeSha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(16 * 1024)
            var read: Int
            while (input.read(buffer).also { read = it } != -1) {
                digest.update(buffer, 0, read)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    /**
     * Проверка совпадения цифрового сертификата подписи между текущим
     * работающим приложением и скачанным обновлением APK.
     */
    fun verifyApkSignature(context: Context, apkFile: File): Boolean {
        return try {
            val currentSignatures = getCurrentSignatures(context)
            if (currentSignatures.isEmpty()) {
                TunnelLog.add("UpdateChecker: не удалось извлечь подпись текущего приложения")
                return false
            }

            val archiveSignatures = getArchiveSignatures(context, apkFile)
            if (archiveSignatures.isEmpty()) {
                TunnelLog.add("UpdateChecker: не удалось прочитать подпись скачанного APK")
                return false
            }

            val matches = currentSignatures.any { curSig ->
                archiveSignatures.any { newSig -> curSig.contentEquals(newSig) }
            }
            matches
        } catch (e: Throwable) {
            TunnelLog.add("UpdateChecker: сбой при проверке сертификата подписи: ${e.message}")
            false
        }
    }

    @Suppress("DEPRECATION")
    private fun getCurrentSignatures(context: Context): List<ByteArray> {
        val pm = context.packageManager
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val info = pm.getPackageInfo(context.packageName, PackageManager.GET_SIGNING_CERTIFICATES)
            info.signingInfo?.apkContentsSigners?.map { it.toByteArray() }
                ?: info.signingInfo?.signingCertificateHistory?.map { it.toByteArray() }
                ?: emptyList()
        } else {
            val info = pm.getPackageInfo(context.packageName, PackageManager.GET_SIGNATURES)
            info.signatures?.map { it.toByteArray() } ?: emptyList()
        }
    }

    @Suppress("DEPRECATION")
    private fun getArchiveSignatures(context: Context, apkFile: File): List<ByteArray> {
        val pm = context.packageManager
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            val info = pm.getPackageArchiveInfo(apkFile.absolutePath, PackageManager.GET_SIGNING_CERTIFICATES)
            info?.signingInfo?.apkContentsSigners?.map { it.toByteArray() }
                ?: info?.signingInfo?.signingCertificateHistory?.map { it.toByteArray() }
                ?: emptyList()
        } else {
            val info = pm.getPackageArchiveInfo(apkFile.absolutePath, PackageManager.GET_SIGNATURES)
            info?.signatures?.map { it.toByteArray() } ?: emptyList()
        }
    }
}
