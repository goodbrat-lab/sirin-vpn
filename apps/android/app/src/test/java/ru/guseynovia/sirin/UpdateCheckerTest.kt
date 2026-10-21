package ru.guseynovia.sirin

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.ByteArrayInputStream
import java.io.File

class UpdateCheckerTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private val testData = "test-checksums-content-123".toByteArray(Charsets.UTF_8)

    private val testSignature = """
-----BEGIN PGP SIGNATURE-----

iQIzBAABCAAdFiEEVcmf8bAzR3lYznxqY7wgXX04B9gFAmrZOYgACgkQY7wgXX04
B9ikiA/+I4YlxMIHhmVaSRmwXa28Plzpkwo43kHyViVgbZHMX7jh9f7MS0wx4jO9
0XtmcKM2ZxqEnVwUnv/JlrjMBr4tGWV27aQtUGxqPnQWpjFAuhPfeWwStlrBD8wL
uifhmiqque/ERf4GFlWKTioT4u6Gi/X+X2Q28RJ7vZEzlclosAchFim0QyT/+hi9
W037TqZDeY9qCbjHTcT7tkYmjeWN+G19tBoIxrCtGJUTNzN9aw96jdlnh72ZdzMO
5UfzbbRyZYMUBgge+VTy487tjTtKSaBiht5jAQqeB70LSdGFBraLhfpDvs294pfP
HcjUWPlG09tmbOYUKxYBwKApFXV8TGw/GjRQMtMBYcvqQtTMhw6GMc5BB9ZYT/AG
gbLVZJCckSLT06plrx+OeniKP6jWRrpDB5T4yTdyz3LMI5queN1/86i9RuD3Ueie
FUJ7uI3kiHj6801J8S0YCEhUAxhbifowVkGElACKCiWS2IdAJrGANPktkvNPnBbp
C6I1YA6+0GlQEVnRFlh1IzxV4KNjyGXheE6mStc+Rwqx0t0Y51jWuV7URjnAc+OD
8I1RhuzWfbVeiwAcFnQlFWFRGcbiduMndaAeuTMy3zQ1ZKpzRuOVphuwJhF7lEjU
1043nAAg78Qf0Hdf6Mhg4CZdpQQfLUuRO4wI0RjHjpVutXjY1h0=
=YXaS
-----END PGP SIGNATURE-----
""".trimIndent().toByteArray(Charsets.UTF_8)

    private fun getPublicKeyStream(): java.io.InputStream {
        val pubKeyFile = File("src/main/res/raw/sirin_pubkey.asc")
        return if (pubKeyFile.exists()) {
            pubKeyFile.inputStream()
        } else {
            val altFile = File("apps/android/app/src/main/res/raw/sirin_pubkey.asc")
            if (altFile.exists()) {
                altFile.inputStream()
            } else {
                throw IllegalStateException("sirin_pubkey.asc not found")
            }
        }
    }

    @Test
    fun testIsNewer() {
        assertTrue(UpdateChecker.isNewer("v1.3.1", "1.3.0"))
        assertTrue(UpdateChecker.isNewer("android-v1.3.1", "1.2.0"))
        assertTrue(UpdateChecker.isNewer("2.0.0", "1.9.9"))
        assertFalse(UpdateChecker.isNewer("1.3.0", "1.3.0"))
        assertFalse(UpdateChecker.isNewer("1.2.0", "1.3.0"))
    }

    @Test
    fun testComputeSha256() {
        val testFile = tempFolder.newFile("test.txt")
        testFile.writeText("hello sirin vpn")

        val sha256 = UpdateChecker.computeSha256(testFile)
        assertEquals("a97f619b74350a86284d35de0972813cb4a286fc9048073175d9017c12fe7500", sha256)
    }

    @Test
    fun testGpgValidSignature() {
        val valid = getPublicKeyStream().use { pkIn ->
            UpdateChecker.verifyGpgSignature(testData, testSignature, pkIn)
        }
        assertTrue("Valid GPG signature must verify successfully", valid)
    }

    @Test
    fun testGpgTamperedDataFails() {
        val tamperedData = "tampered-checksums-content".toByteArray(Charsets.UTF_8)
        val valid = getPublicKeyStream().use { pkIn ->
            UpdateChecker.verifyGpgSignature(tamperedData, testSignature, pkIn)
        }
        assertFalse("Tampered data must fail GPG verification", valid)
    }

    @Test
    fun testGpgCorruptSignatureFails() {
        val corruptSig = "not-a-valid-gpg-signature".toByteArray(Charsets.UTF_8)
        val valid = getPublicKeyStream().use { pkIn ->
            UpdateChecker.verifyGpgSignature(testData, corruptSig, pkIn)
        }
        assertFalse("Corrupt GPG signature must fail verification", valid)
    }
}
