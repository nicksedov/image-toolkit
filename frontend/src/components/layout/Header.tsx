export function Header() {
  return (
    <header className="border-b bg-gradient-to-r from-blue-600 to-indigo-700 text-white">
      <div className="mx-auto max-w-7xl px-4 py-4 sm:px-6">
        <h1 className="text-2xl font-bold tracking-tight">Image Dedup</h1>
        <p className="text-sm text-blue-100">
          Manage duplicate images in your media library
        </p>
      </div>
    </header>
  )
}
