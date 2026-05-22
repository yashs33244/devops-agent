import { Nav } from "@/components/Nav";
import { Hero } from "@/components/Hero";
import { PainSection } from "@/components/PainSection";
import { BentoSection } from "@/components/BentoSection";
import { Integrations } from "@/components/Integrations";
import { HowItWorks } from "@/components/HowItWorks";
import { SecuritySection } from "@/components/SecuritySection";
import { FAQSection } from "@/components/FAQSection";
import { CTASection } from "@/components/CTASection";
import { Footer } from "@/components/Footer";
import { PixBar } from "@/components/PixBar";

export default function Home() {
  return (
    <>
      <Nav />
      <Hero />
      <PixBar />
      <PainSection />
      <PixBar variant="cream" />
      <BentoSection />
      <PixBar variant="flip" />
      <Integrations />
      <PixBar />
      <HowItWorks />
      <SecuritySection />
      <PixBar variant="flip" />
      <FAQSection />
      <CTASection />
      <Footer />
    </>
  );
}
