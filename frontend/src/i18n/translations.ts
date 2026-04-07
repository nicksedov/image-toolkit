export const translations = {
  en: {
    // Header
    "header.title": "Image Dedup",
    "header.subtitle": "Manage duplicate images in your media library",
    "header.toggleTheme": "Toggle theme",
    "header.toggleLanguage": "Toggle language",

    // Tabs
    "tabs.settings": "Settings",
    "tabs.gallery": "Gallery",
    "tabs.deduplication": "Deduplication",

    // Loading
    "common.loading": "Loading...",
    "common.cancel": "Cancel",

    // Settings tab
    "settings.title": "Gallery",
    "settings.description": "Manage the folders included in your image gallery. Adding a folder will automatically start scanning it for images.",
    "settings.rescanAll": "Rescan All",
    "settings.folderCount": "{count} folder(s) in gallery",
    "settings.folderCountOne": "{count} folder in gallery",
    "settings.toastScanComplete": "Scan complete!",
    "settings.toastRescanStarted": "Rescan started",
    "settings.toastRescanComplete": "Rescan complete!",
    "settings.toastNoFolders": "No folders in the gallery to scan",
    "settings.toastAddFailed": "Failed to add folder",
    "settings.toastRemoveFailed": "Failed to remove folder",
    "settings.toastRescanFailed": "Failed to start rescan",
    "settings.toastFilesRemoved": "{message} ({count} files removed)",

    // Add folder form
    "addFolder.placeholder": "Enter folder path, e.g. C:\\Photos or /home/user/photos",
    "addFolder.button": "Add Folder",

    // Folder list
    "folderList.loading": "Loading gallery folders...",
    "folderList.empty": "No folders in the gallery",
    "folderList.emptyHint": "Add a folder above to start scanning images.",
    "folderList.files": "{count} files",
    "folderList.added": "Added: {date}",
    "folderList.removeTitle": "Remove Folder",
    "folderList.removeDescription": "Are you sure you want to remove this folder from the gallery? All indexed files from this folder will be removed from the database. The actual files on disk will NOT be deleted.",
    "folderList.removeButton": "Remove",
    "folderList.removing": "Removing...",

    // Gallery tab
    "gallery.imageCount": "{count} image(s) in gallery",
    "gallery.imageCountOne": "{count} image in gallery",
    "gallery.thumbnails": "Thumbnails",
    "gallery.list": "List",
    "gallery.empty": "No images in the gallery",
    "gallery.emptyHint": "Add folders in the Settings tab to start browsing images.",
    "gallery.loadingMore": "Loading more images...",
    "gallery.allLoaded": "All {count} images loaded",
    "gallery.noPreview": "No preview",

    // Gallery image list table
    "galleryList.fileName": "File Name",
    "galleryList.directory": "Directory",
    "galleryList.size": "Size",
    "galleryList.modified": "Modified",

    // Image lightbox
    "lightbox.title": "Image preview",
    "lightbox.alt": "Full size preview",

    // Thumbnail
    "thumbnail.alt": "Thumbnail",

    // Toolbar
    "toolbar.rescan": "Rescan",
    "toolbar.resetSelection": "Reset Selection",
    "toolbar.generateScript": "Generate Script",
    "toolbar.deleteSelected": "Delete Selected",
    "toolbar.batchDedup": "Batch Dedup",
    "toolbar.filesSelected": "{count} file(s) selected",
    "toolbar.filesSelectedOne": "{count} file selected",
    "toolbar.groupsPerPage": "Groups per page:",

    // Deduplication tab
    "dedup.toastScanStarted": "Scan started",
    "dedup.toastScanComplete": "Scan complete!",
    "dedup.toastSelectFile": "Please select at least one file.",
    "dedup.toastScanFailed": "Failed to start scan",

    // Duplicate group card
    "duplicateGroup.title": "Group #{index}",
    "duplicateGroup.files": "{count} files",
    "duplicateGroup.sizeEach": "{size} each",
    "duplicateGroup.md5": "MD5: {hash}",

    // File item
    "fileItem.selectFolder": "Click to select all files from this folder",
    "fileItem.modified": "Modified: {date}",

    // Empty state
    "emptyState.title": "No Duplicates Found",
    "emptyState.description": "Your media library appears to be clean, or you need to run a scan first.",

    // Scan progress
    "scanProgress.scanning": "Scanning in progress...",
    "scanProgress.filesProcessed": "{count} files processed",

    // Pagination
    "pagination.first": "First",
    "pagination.prev": "Prev",
    "pagination.next": "Next",
    "pagination.last": "Last",
    "pagination.pageInfo": "Page {current} of {total}",

    // Generate script modal
    "generateScript.title": "Generate Removal Script",
    "generateScript.description": "Generate a script to move {count} selected file(s) to a trash directory.",
    "generateScript.scriptType": "Script type",
    "generateScript.windows": "Windows (PowerShell .ps1)",
    "generateScript.bash": "Linux/macOS (Bash .sh)",
    "generateScript.outputDir": "Output directory for script",
    "generateScript.outputPlaceholder": "C:\\path\\to\\output",
    "generateScript.trashDir": "Trash directory (where files will be moved)",
    "generateScript.trashPlaceholder": "C:\\path\\to\\trash (optional)",
    "generateScript.hint": "The script will move selected files to the trash directory. Review the script before running it.",
    "generateScript.button": "Generate Script",
    "generateScript.generating": "Generating...",
    "generateScript.errorOutputDir": "Please specify an output directory for the script.",
    "generateScript.errorFailed": "Failed to generate script",
    "generateScript.success": "Script generated successfully! Saved to: {path}",

    // Delete files modal
    "deleteFiles.title": "Delete Selected Files",
    "deleteFiles.description": "This action will delete {count} file(s).",
    "deleteFiles.warning": "Warning: Deleted files cannot be easily recovered unless you specify a trash directory.",
    "deleteFiles.trashDir": "Trash directory (optional)",
    "deleteFiles.trashPlaceholder": "C:\\path\\to\\trash (leave empty to delete permanently)",
    "deleteFiles.hint": "If a trash directory is specified, files will be moved there. Otherwise, files will be permanently deleted.",
    "deleteFiles.button": "Delete Files",
    "deleteFiles.deleting": "Deleting...",
    "deleteFiles.confirmPermanent": "No trash directory specified. Files will be PERMANENTLY deleted. Continue?",
    "deleteFiles.success": "Successfully deleted {count} file(s).",
    "deleteFiles.successWithFailed": "Successfully deleted {count} file(s). Failed: {failed}.",
    "deleteFiles.errorFailed": "Failed to delete files",

    // Batch dedup modal
    "batchDedup.title": "Batch Deduplication",
    "batchDedup.description": "Select which folder should keep the file for each pattern. Files in other folders will be deleted.",
    "batchDedup.groups": "{count} groups",
    "batchDedup.files": "{count} files",
    "batchDedup.noPatterns": "No folder patterns found.",
    "batchDedup.trashDir": "Trash directory (optional)",
    "batchDedup.trashPlaceholder": "C:\\path\\to\\trash (leave empty to delete permanently)",
    "batchDedup.applyRules": "Apply Rules",
    "batchDedup.applying": "Applying...",
    "batchDedup.errorNoRules": "Please select at least one folder to keep.",
    "batchDedup.confirmApply": "This will apply {count} rule(s) to delete duplicate files. Continue?",
    "batchDedup.success": "Successfully deleted {count} file(s).",
    "batchDedup.successWithFailed": "Successfully deleted {count} file(s). Failed: {failed}.",
    "batchDedup.errorFailed": "Failed to apply batch rules",
  },

  ru: {
    // Header
    "header.title": "Image Dedup",
    "header.subtitle": "Управление дубликатами изображений в медиатеке",
    "header.toggleTheme": "Переключить тему",
    "header.toggleLanguage": "Переключить язык",

    // Tabs
    "tabs.settings": "Настройки",
    "tabs.gallery": "Галерея",
    "tabs.deduplication": "Дедупликация",

    // Loading
    "common.loading": "Загрузка...",
    "common.cancel": "Отмена",

    // Settings tab
    "settings.title": "Галерея",
    "settings.description": "Управление папками, включёнными в вашу галерею изображений. Добавление папки автоматически запустит сканирование.",
    "settings.rescanAll": "Сканировать все",
    "settings.folderCount": "{count} папок в галерее",
    "settings.folderCountOne": "{count} папка в галерее",
    "settings.toastScanComplete": "Сканирование завершено!",
    "settings.toastRescanStarted": "Повторное сканирование начато",
    "settings.toastRescanComplete": "Повторное сканирование завершено!",
    "settings.toastNoFolders": "В галерее нет папок для сканирования",
    "settings.toastAddFailed": "Не удалось добавить папку",
    "settings.toastRemoveFailed": "Не удалось удалить папку",
    "settings.toastRescanFailed": "Не удалось начать сканирование",
    "settings.toastFilesRemoved": "{message} ({count} файлов удалено)",

    // Add folder form
    "addFolder.placeholder": "Введите путь к папке, напр. C:\\Фото или /home/user/photos",
    "addFolder.button": "Добавить папку",

    // Folder list
    "folderList.loading": "Загрузка папок галереи...",
    "folderList.empty": "В галерее нет папок",
    "folderList.emptyHint": "Добавьте папку выше, чтобы начать сканирование изображений.",
    "folderList.files": "{count} файлов",
    "folderList.added": "Добавлено: {date}",
    "folderList.removeTitle": "Удалить папку",
    "folderList.removeDescription": "Вы уверены, что хотите удалить эту папку из галереи? Все проиндексированные файлы этой папки будут удалены из базы данных. Файлы на диске НЕ будут удалены.",
    "folderList.removeButton": "Удалить",
    "folderList.removing": "Удаление...",

    // Gallery tab
    "gallery.imageCount": "{count} изображений в галерее",
    "gallery.imageCountOne": "{count} изображение в галерее",
    "gallery.thumbnails": "Миниатюры",
    "gallery.list": "Список",
    "gallery.empty": "В галерее нет изображений",
    "gallery.emptyHint": "Добавьте папки на вкладке Настройки, чтобы начать просмотр изображений.",
    "gallery.loadingMore": "Загрузка изображений...",
    "gallery.allLoaded": "Все {count} изображений загружены",
    "gallery.noPreview": "Нет превью",

    // Gallery image list table
    "galleryList.fileName": "Имя файла",
    "galleryList.directory": "Директория",
    "galleryList.size": "Размер",
    "galleryList.modified": "Изменён",

    // Image lightbox
    "lightbox.title": "Просмотр изображения",
    "lightbox.alt": "Полноразмерный просмотр",

    // Thumbnail
    "thumbnail.alt": "Миниатюра",

    // Toolbar
    "toolbar.rescan": "Сканировать",
    "toolbar.resetSelection": "Сбросить выбор",
    "toolbar.generateScript": "Создать скрипт",
    "toolbar.deleteSelected": "Удалить выбранные",
    "toolbar.batchDedup": "Пакетная дедупликация",
    "toolbar.filesSelected": "{count} файлов выбрано",
    "toolbar.filesSelectedOne": "{count} файл выбран",
    "toolbar.groupsPerPage": "Групп на странице:",

    // Deduplication tab
    "dedup.toastScanStarted": "Сканирование начато",
    "dedup.toastScanComplete": "Сканирование завершено!",
    "dedup.toastSelectFile": "Выберите хотя бы один файл.",
    "dedup.toastScanFailed": "Не удалось начать сканирование",

    // Duplicate group card
    "duplicateGroup.title": "Группа #{index}",
    "duplicateGroup.files": "{count} файлов",
    "duplicateGroup.sizeEach": "{size} каждый",
    "duplicateGroup.md5": "MD5: {hash}",

    // File item
    "fileItem.selectFolder": "Нажмите, чтобы выбрать все файлы из этой папки",
    "fileItem.modified": "Изменён: {date}",

    // Empty state
    "emptyState.title": "Дубликаты не найдены",
    "emptyState.description": "Ваша медиатека чиста, или нужно сначала запустить сканирование.",

    // Scan progress
    "scanProgress.scanning": "Сканирование...",
    "scanProgress.filesProcessed": "{count} файлов обработано",

    // Pagination
    "pagination.first": "Первая",
    "pagination.prev": "Назад",
    "pagination.next": "Далее",
    "pagination.last": "Последняя",
    "pagination.pageInfo": "Страница {current} из {total}",

    // Generate script modal
    "generateScript.title": "Создание скрипта удаления",
    "generateScript.description": "Создать скрипт для перемещения {count} выбранных файлов в папку корзины.",
    "generateScript.scriptType": "Тип скрипта",
    "generateScript.windows": "Windows (PowerShell .ps1)",
    "generateScript.bash": "Linux/macOS (Bash .sh)",
    "generateScript.outputDir": "Директория для сохранения скрипта",
    "generateScript.outputPlaceholder": "C:\\путь\\к\\папке",
    "generateScript.trashDir": "Директория корзины (куда будут перемещены файлы)",
    "generateScript.trashPlaceholder": "C:\\путь\\к\\корзине (необязательно)",
    "generateScript.hint": "Скрипт переместит выбранные файлы в директорию корзины. Проверьте скрипт перед запуском.",
    "generateScript.button": "Создать скрипт",
    "generateScript.generating": "Создание...",
    "generateScript.errorOutputDir": "Укажите директорию для сохранения скрипта.",
    "generateScript.errorFailed": "Не удалось создать скрипт",
    "generateScript.success": "Скрипт успешно создан! Сохранён в: {path}",

    // Delete files modal
    "deleteFiles.title": "Удаление выбранных файлов",
    "deleteFiles.description": "Это действие удалит {count} файлов.",
    "deleteFiles.warning": "Внимание: удалённые файлы невозможно легко восстановить, если не указана директория корзины.",
    "deleteFiles.trashDir": "Директория корзины (необязательно)",
    "deleteFiles.trashPlaceholder": "C:\\путь\\к\\корзине (оставьте пустым для безвозвратного удаления)",
    "deleteFiles.hint": "Если указана директория корзины, файлы будут перемещены туда. Иначе файлы будут удалены безвозвратно.",
    "deleteFiles.button": "Удалить файлы",
    "deleteFiles.deleting": "Удаление...",
    "deleteFiles.confirmPermanent": "Директория корзины не указана. Файлы будут БЕЗВОЗВРАТНО удалены. Продолжить?",
    "deleteFiles.success": "Успешно удалено {count} файлов.",
    "deleteFiles.successWithFailed": "Успешно удалено {count} файлов. Ошибок: {failed}.",
    "deleteFiles.errorFailed": "Не удалось удалить файлы",

    // Batch dedup modal
    "batchDedup.title": "Пакетная дедупликация",
    "batchDedup.description": "Выберите папку, в которой нужно сохранить файл для каждого шаблона. Файлы в других папках будут удалены.",
    "batchDedup.groups": "{count} групп",
    "batchDedup.files": "{count} файлов",
    "batchDedup.noPatterns": "Шаблоны папок не найдены.",
    "batchDedup.trashDir": "Директория корзины (необязательно)",
    "batchDedup.trashPlaceholder": "C:\\путь\\к\\корзине (оставьте пустым для безвозвратного удаления)",
    "batchDedup.applyRules": "Применить правила",
    "batchDedup.applying": "Применение...",
    "batchDedup.errorNoRules": "Выберите хотя бы одну папку для сохранения.",
    "batchDedup.confirmApply": "Это применит {count} правил для удаления дубликатов. Продолжить?",
    "batchDedup.success": "Успешно удалено {count} файлов.",
    "batchDedup.successWithFailed": "Успешно удалено {count} файлов. Ошибок: {failed}.",
    "batchDedup.errorFailed": "Не удалось применить пакетные правила",
  },
} as const
