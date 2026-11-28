import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { Toaster } from "@/components/ui/toaster";
import { TooltipProvider } from "@/components/ui/tooltip";
import Index from "./pages/Index.tsx";
import Solutions from "./pages/Solutions.tsx";
import Docs from "./pages/Docs.tsx";
import Calculator from "./pages/Calculator.tsx";
import RiskAssessment from "./pages/RiskAssessment.tsx";
import Privacy from "./pages/Privacy.tsx";
import About from "./pages/About.tsx";
import VsPresidio from "./pages/VsPresidio.tsx";
import RedactPiiClaudeMcp from "./pages/guides/RedactPiiClaudeMcp.tsx";
import GdprEuAiActRedaction from "./pages/guides/GdprEuAiActRedaction.tsx";
import NotFound from "./pages/NotFound.tsx";

const queryClient = new QueryClient();

const App = () => (
  <QueryClientProvider client={queryClient}>
    <TooltipProvider>
      <Toaster />
      <Sonner />
      <BrowserRouter>
        <Routes>
        <Route path="/" element={<Index />} />
        <Route path="/solutions" element={<Solutions />} />
        <Route path="/docs/*" element={<Docs />} />
        <Route path="/calculator" element={<Calculator />} />
        <Route path="/risk-assessment" element={<RiskAssessment />} />
        <Route path="/privacy" element={<Privacy />} />
        <Route path="/about" element={<About />} />
        <Route path="/vs-microsoft-presidio" element={<VsPresidio />} />
        <Route path="/guides/redact-pii-claude-mcp" element={<RedactPiiClaudeMcp />} />
        <Route path="/guides/local-pii-redaction-gdpr-eu-ai-act" element={<GdprEuAiActRedaction />} />
        {/* ADD ALL CUSTOM ROUTES ABOVE THE CATCH-ALL "*" ROUTE */}
          <Route path="*" element={<NotFound />} />
        </Routes>
      </BrowserRouter>
    </TooltipProvider>
  </QueryClientProvider>
);

export default App;
