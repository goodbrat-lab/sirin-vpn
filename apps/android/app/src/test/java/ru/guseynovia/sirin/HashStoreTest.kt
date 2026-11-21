package ru.guseynovia.sirin

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class HashStoreTest {

    @Test
    fun normalizeVkLinksWithAndWithoutScheme() {
        assertEquals("abcdef123456", HashStore.normalize("https://vk.com/call/join/abcdef123456"))
        assertEquals("abcdef123456", HashStore.normalize("http://vk.ru/call/join/abcdef123456?extra=1#frag"))
        assertEquals("abcdef123456", HashStore.normalize("vk.com/call/join/abcdef123456"))
        assertEquals("abcdef123456", HashStore.normalize("vk.me/call/join/abcdef123456/"))
        assertEquals("abcdef123456", HashStore.normalize("calls.vk.com/call/join/abcdef123456"))
        assertEquals("abcdef123456", HashStore.normalize("vk://vk.com/call/join/abcdef123456"))
        assertEquals("abcdef123456", HashStore.normalize("https://vk.me/join/abcdef123456"))
        assertEquals("abcdef123456", HashStore.normalize("https://calls.vk.com/call/abcdef123456"))
        assertEquals("abcdef123456", HashStore.normalize("(https://vk.com/call/join/abcdef123456)."))
    }

    @Test
    fun normalizeBareHashAndRejectNonHashes() {
        assertEquals("o5_x1Y2z-99", HashStore.normalize("o5_x1Y2z-99"))
        assertNull(HashStore.normalize("https://example.com/some/path"))
        assertNull(HashStore.normalize("Присоединяйся"))
        assertNull(HashStore.normalize("VK:"))
        assertNull(HashStore.normalize(""))
    }

    @Test
    fun parseManyExtractsLinksFromInvitationText() {
        val text = "Присоединяйся к звонку в VK: https://vk.com/call/join/AbCdEf123456, ждём!"
        assertEquals(listOf("AbCdEf123456"), HashStore.parseMany(text))

        val englishText = "Join my VK call: vk.com/call/join/XyZ789_012"
        assertEquals(listOf("XyZ789_012"), HashStore.parseMany(englishText))

        val bareHashes = "hash_one1, hash_two2; hash_three3"
        assertEquals(listOf("hash_one1", "hash_two2", "hash_three3"), HashStore.parseMany(bareHashes))
    }

    @Test
    fun parseDionDoesNotSwallowVkCallLinks() {
        assertEquals(emptyList<String>(), HashStore.parseDion("https://vk.com/call/join/AbCdEf123456"))
        assertEquals(listOf("dion:abc-def-ghi"), HashStore.parseDion("https://dion.vc/event/abc-def-ghi"))
    }
}
