package ru.guseynovia.sirin

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import androidx.compose.runtime.mutableStateOf
import java.net.Inet4Address

/**
 * Доверенные Wi-Fi сети (например, домашний роутер с Sirin VPN).
 *
 * Идентификация выполняется БЕЗ разрешения ACCESS_FINE_LOCATION — по отпечатку
 * IPv4-шлюза и DNS-серверов Wi-Fi интерфейса из LinkProperties.
 */
object TrustedWifi {
    private const val PREFS = "sirin_trusted_wifi"
    private const val K_ENABLED = "enabled"
    private const val K_ENTRIES = "entries"

    val enabled = mutableStateOf(false)

    /** Карта сохранённых сетей: id ("wifi:gw|dns") -> человекочитаемая подпись. */
    val saved = mutableStateOf<Map<String, String>>(emptyMap())

    private var appCtx: Context? = null
    private var loaded = false

    @Synchronized
    fun attach(ctx: Context) {
        if (loaded) return
        loaded = true
        appCtx = ctx.applicationContext
        val p = prefs(ctx) ?: return
        enabled.value = p.getBoolean(K_ENABLED, false)
        val raw = p.getStringSet(K_ENTRIES, emptySet()) ?: emptySet()
        val parsed = linkedMapOf<String, String>()
        for (line in raw) {
            val sep = line.indexOf('\t')
            if (sep > 0) {
                parsed[line.substring(0, sep)] = line.substring(sep + 1)
            }
        }
        saved.value = parsed
    }

    fun setEnabled(ctx: Context, value: Boolean) {
        attach(ctx)
        enabled.value = value
        prefs(ctx)?.edit()?.putBoolean(K_ENABLED, value)?.apply()
    }

    /**
     * Вычисляет стабильный отпечаток текущего Wi-Fi (шлюз IPv4 + DNS)
     * без запроса геолокации. Возвращает Pair(id, label) или null, если Wi-Fi не активен.
     */
    fun currentWifiFingerprint(ctx: Context): Pair<String, String>? {
        return try {
            val cm = ctx.getSystemService(ConnectivityManager::class.java) ?: return null
            @Suppress("DEPRECATION")
            val wifiNet = cm.allNetworks.firstOrNull { net ->
                val caps = cm.getNetworkCapabilities(net) ?: return@firstOrNull false
                caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) &&
                    caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_VPN)
            } ?: return null

            val lp = cm.getLinkProperties(wifiNet) ?: return null
            val gw = lp.routes
                .firstOrNull { it.isDefaultRoute && it.gateway is Inet4Address }
                ?.gateway?.hostAddress
                ?: lp.linkAddresses
                    .mapNotNull { (it.address as? Inet4Address)?.hostAddress }
                    .firstOrNull()
                    ?.substringBeforeLast('.')
                    ?.plus(".1")
                ?: return null

            val dns = lp.dnsServers
                .filterIsInstance<Inet4Address>()
                .mapNotNull { it.hostAddress }
                .sorted()
                .joinToString(",")

            val id = "wifi:$gw|$dns"
            val label = "Wi-Fi ($gw)"
            id to label
        } catch (_: Throwable) {
            null
        }
    }

    fun isOnTrustedWifi(ctx: Context): Boolean {
        if (!enabled.value || saved.value.isEmpty()) return false
        val fp = currentWifiFingerprint(ctx) ?: return false
        return fp.first in saved.value
    }

    /** Запоминает текущий Wi-Fi. Возвращает подпись сети или null, если нет Wi-Fi. */
    fun addCurrent(ctx: Context): String? {
        attach(ctx)
        val (id, label) = currentWifiFingerprint(ctx) ?: return null
        val next = LinkedHashMap(saved.value)
        next[id] = label
        saved.value = next
        saveEntries(ctx, next)
        return label
    }

    fun remove(ctx: Context, id: String) {
        attach(ctx)
        val next = LinkedHashMap(saved.value)
        next.remove(id)
        saved.value = next
        saveEntries(ctx, next)
    }

    private fun saveEntries(ctx: Context, map: Map<String, String>) {
        val encoded = map.entries.map { "${it.key}\t${it.value}" }.toSet()
        prefs(ctx)?.edit()?.putStringSet(K_ENTRIES, HashSet(encoded))?.apply()
    }

    private fun prefs(ctx: Context) = try {
        ctx.applicationContext.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
    } catch (_: Throwable) {
        null
    }
}
