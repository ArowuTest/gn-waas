import { Droplets, Github } from 'lucide-react'

export default function Footer() {
  return (
    <footer className="bg-gray-900 text-white py-12">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex flex-col md:flex-row items-center justify-between gap-6">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-green-700 rounded-lg flex items-center justify-center">
              <Droplets className="w-5 h-5 text-yellow-400" />
            </div>
            <div>
              <div className="font-bold text-white">GN-WAAS</div>
              <div className="text-xs text-gray-400">Ghana National Water Audit & Assurance System</div>
            </div>
          </div>

          <div className="flex items-center gap-6 text-sm text-gray-400">
            <span>Built for Ghana. Sovereign by design.</span>
            <a
              href="https://github.com/ArowuTest/gn-waas"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1.5 hover:text-white transition-colors"
            >
              <Github className="w-4 h-4" />
              GitHub
            </a>
          </div>

          <div className="text-xs text-gray-500">
            © 2026 ArowuTest. NITA-certified. GRA-compliant.
          </div>
        </div>
      </div>
    </footer>
  )
}
