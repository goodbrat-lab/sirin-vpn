package ru.guseynovia.sirin

import com.journeyapps.barcodescanner.ScanOptions

/**
 * Вспомогательный объект настройки параметров сканирования QR-кодов ключей Sirin.
 */
object QrScanner {
    fun options(): ScanOptions = ScanOptions().apply {
        setDesiredBarcodeFormats(ScanOptions.QR_CODE)
        setPrompt("Наведите камеру на QR-код с ключом Sirin VPN")
        setCameraId(0)
        setBeepEnabled(false)
        setBarcodeImageEnabled(false)
        setOrientationLocked(false)
    }
}
