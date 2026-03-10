import Navbar from './components/Navbar'
import Hero from './components/Hero'
import PortalAccess from './components/PortalAccess'
import Problem from './components/Problem'
import Solution from './components/Solution'
import HowItWorks from './components/HowItWorks'
import Features from './components/Features'
import BusinessModel from './components/BusinessModel'
import Compliance from './components/Compliance'
import ContactForm from './components/ContactForm'
import Footer from './components/Footer'

export default function App() {
  return (
    <div className="min-h-screen bg-white">
      <Navbar />
      <Hero />
      <PortalAccess />
      <Problem />
      <Solution />
      <HowItWorks />
      <Features />
      <BusinessModel />
      <Compliance />
      <ContactForm />
      <Footer />
    </div>
  )
}
